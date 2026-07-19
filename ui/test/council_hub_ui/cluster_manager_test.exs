defmodule CouncilHubUi.ClusterManagerTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.ClusterManager

  # Each test gets an isolated manager: unique process name + temp peers file.
  setup do
    path = Path.join(System.tmp_dir!(), "ch_peers_#{System.unique_integer([:positive])}")
    name = :"clmgr_#{System.unique_integer([:positive])}"
    on_exit(fn -> File.rm(path) end)
    {:ok, _pid} = ClusterManager.start_link(name: name, path: path)
    %{name: name, path: path}
  end

  describe "managed_peers persistence" do
    test "loads peers from an existing file on boot", %{path: path} do
      File.write!(path, "council_hub@10.0.0.1\ncouncil_hub@10.0.0.2\n")
      name = :"clmgr_boot_#{System.unique_integer([:positive])}"
      {:ok, _} = ClusterManager.start_link(name: name, path: path)

      assert Enum.sort(ClusterManager.managed_peers(name)) ==
               ["council_hub@10.0.0.1", "council_hub@10.0.0.2"]
    end

    test "starts empty when no file exists", %{name: name} do
      assert ClusterManager.managed_peers(name) == []
    end
  end

  describe "connect validation" do
    test "rejects a malformed node name", %{name: name} do
      assert {:error, msg} = ClusterManager.connect(name, "not-a-node")
      assert msg =~ "invalid node name"
    end

    test "rejects connecting to self", %{name: name} do
      assert {:error, msg} = ClusterManager.connect(name, to_string(Node.self()))
      assert msg =~ "this node"
    end

    test "reports when the node is not distributed", %{name: name} do
      # The test VM runs as :nonode@nohost, so Node.connect returns :ignored.
      assert {:error, msg} = ClusterManager.connect(name, "council_hub@10.0.0.9")
      assert msg =~ "distributed mode"
    end
  end

  describe "disconnect" do
    test "drops a peer from the persisted set", %{path: path} do
      File.write!(path, "council_hub@10.0.0.1\n")
      boot = :"clmgr_disc_#{System.unique_integer([:positive])}"
      {:ok, _} = ClusterManager.start_link(name: boot, path: path)
      assert ClusterManager.managed_peers(boot) == ["council_hub@10.0.0.1"]

      assert :ok = ClusterManager.disconnect(boot, "council_hub@10.0.0.1")
      assert ClusterManager.managed_peers(boot) == []
      assert File.read!(path) == ""
    end
  end

  describe "self-heal known set" do
    test "seeds the keep-alive set from the persisted peers on boot", %{path: path} do
      File.write!(path, "council_hub@10.0.0.1\ncouncil_hub@10.0.0.2\n")
      name = :"clmgr_known_#{System.unique_integer([:positive])}"
      {:ok, _} = ClusterManager.start_link(name: name, path: path)

      assert Enum.sort(ClusterManager.known_peers(name)) ==
               ["council_hub@10.0.0.1", "council_hub@10.0.0.2"]
    end

    test "learns a peer from a :nodeup event", %{name: name} do
      send(Process.whereis(name), {:nodeup, :"council_hub@10.0.0.7"})
      # known_peers is a call, so it serializes behind the info message above.
      assert "council_hub@10.0.0.7" in ClusterManager.known_peers(name)
    end

    test "an explicit disconnect also drops the peer from the keep-alive set", %{path: path} do
      name = :"clmgr_known_disc_#{System.unique_integer([:positive])}"
      {:ok, _} = ClusterManager.start_link(name: name, path: path)
      send(Process.whereis(name), {:nodeup, :"council_hub@10.0.0.8"})
      assert "council_hub@10.0.0.8" in ClusterManager.known_peers(name)

      assert :ok = ClusterManager.disconnect(name, "council_hub@10.0.0.8")
      refute "council_hub@10.0.0.8" in ClusterManager.known_peers(name)
    end

    test "the reconnect tick runs without crashing", %{path: path} do
      # Short interval so the timer fires during the test; the manager should
      # stay responsive after re-dialing its (unreachable) known peers.
      File.write!(path, "council_hub@10.0.0.9\n")
      name = :"clmgr_tick_#{System.unique_integer([:positive])}"
      {:ok, pid} = ClusterManager.start_link(name: name, path: path, reconnect_interval: 20)
      Process.sleep(60)
      assert Process.alive?(pid)
      assert "council_hub@10.0.0.9" in ClusterManager.known_peers(name)
    end
  end
end
