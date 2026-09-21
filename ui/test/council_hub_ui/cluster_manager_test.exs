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

  describe "own-identity drift" do
    test "ip_status/1 reports no drift when not distributed", %{name: name} do
      # mix test runs as :nonode@nohost, so NodeIdentity.status/0 short-circuits
      # to a non-drifted status without ever comparing addresses.
      assert ClusterManager.ip_status(name) == %{
               registered: nil,
               current: nil,
               drifted?: false,
               checkable?: false,
               self_heal_supported?: true
             }
    end

    test "self-heal capability is read at boot, not learned by failing", %{name: name} do
      # The test VM started no distribution, so nothing blocks a rebind. The
      # point is that the answer is present before any drift is ever seen —
      # under RELEASE_DISTRIBUTION=name this is false from the first tick, so
      # the loop never attempts (or logs) a structurally impossible rebind.
      assert %{self_heal_supported?: supported?} = ClusterManager.ip_status(name)
      assert supported? == not CouncilHubUi.NodeIdentity.static_distribution?()
    end

    test "the reconnect tick keeps ip_status fresh without attempting a rebind", %{path: path} do
      name = :"clmgr_ip_tick_#{System.unique_integer([:positive])}"
      {:ok, pid} = ClusterManager.start_link(name: name, path: path, reconnect_interval: 20)
      Process.sleep(60)

      # Still alive and still non-distributed — a real self-heal rebind would
      # attempt to start distribution, which is unsafe to trigger in the test
      # VM, so this asserts the tick correctly treats "not distributed" as
      # "not drifted" and never reaches that path.
      assert Process.alive?(pid)

      assert ClusterManager.ip_status(name) == %{
               registered: nil,
               current: nil,
               drifted?: false,
               checkable?: false,
               self_heal_supported?: true
             }
    end
  end

  describe "seed doctor" do
    test "logs a stale peer name once, not on every probe round" do
      # capture_log is global and this suite is async, so the address here is
      # deliberately unique — a sibling test's identical sentence would
      # otherwise be counted as a repeat.
      probe = fn "10.9.9.9" -> {:ok, "peer@10.9.9.8"} end

      log =
        ExUnit.CaptureLog.capture_log(fn ->
          name = start_manager(seeds: "10.9.9.9", seed_probe: probe, seed_probe_interval: 0)

          eventually(fn -> ClusterManager.seed_status(name).findings end)
          Process.sleep(80)
        end)

      occurrences = log |> String.split("peer@10.9.9.8") |> length()
      assert occurrences == 2, "expected exactly one stale-name log line, got #{occurrences - 1}"
    end

    test "reports a seed that is up but hands back a node name for another address" do
      probe = fn "192.168.0.4" -> {:ok, "bob@192.168.0.5"} end
      name = start_manager(seeds: "192.168.0.4", seed_probe: probe)

      assert [finding] = eventually(fn -> ClusterManager.seed_status(name).findings end)
      assert finding.status == :stale_name
      assert ClusterManager.seed_status(name).checked_at != nil
    end

    test "does not probe when no seeds are configured" do
      probe = fn _ -> flunk("must not probe without seeds") end
      name = start_manager(seeds: "", seed_probe: probe)

      Process.sleep(50)
      assert ClusterManager.seed_status(name) == %{findings: [], checked_at: nil}
    end

    test "a slow probe does not stall the reconnect tick" do
      parent = self()

      probe = fn _ ->
        send(parent, :probe_started)
        Process.sleep(300)
        {:error, :timeout}
      end

      name = start_manager(seeds: "192.168.0.4", seed_probe: probe)
      assert_receive :probe_started, 500

      # The manager answers while the probe is still in flight.
      assert ClusterManager.seed_status(name).findings == []
      assert ClusterManager.known_peers(name) == []
    end
  end

  describe "advertised address probe" do
    test "caches a failed self-probe so /status and /health can report it" do
      unreachable = %{
        host: "192.168.0.10",
        port: 4369,
        checkable?: true,
        reachable?: false,
        warning: "nothing answers on 192.168.0.10:4369"
      }

      name = start_manager(advertised_check: fn -> unreachable end)

      status = eventually_map(fn -> ClusterManager.advertised_status(name) end)
      assert status.warning =~ "nothing answers"
      refute status.reachable?
    end

    test "reports nothing to say when the node is not distributed" do
      name = start_manager([])

      # The test VM is :nonode@nohost, so the real check is not applicable.
      status = ClusterManager.advertised_status(name)
      refute status.checkable?
      assert status.warning == nil
    end
  end

  ## Helpers

  defp start_manager(opts) do
    path = Path.join(System.tmp_dir!(), "ch_peers_#{System.unique_integer([:positive])}")
    name = :"clmgr_seed_#{System.unique_integer([:positive])}"
    on_exit(fn -> File.rm(path) end)

    {:ok, _pid} =
      ClusterManager.start_link([name: name, path: path, reconnect_interval: 10] ++ opts)

    name
  end

  defp eventually_map(fun, attempts \\ 50) do
    case fun.() do
      %{warning: nil} when attempts > 0 ->
        Process.sleep(20)
        eventually_map(fun, attempts - 1)

      result ->
        result
    end
  end

  defp eventually(fun, attempts \\ 50) do
    case fun.() do
      [] when attempts > 0 ->
        Process.sleep(20)
        eventually(fun, attempts - 1)

      result ->
        result
    end
  end
end
