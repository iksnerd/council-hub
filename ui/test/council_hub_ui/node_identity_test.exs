defmodule CouncilHubUi.NodeIdentityTest do
  # async: false — the drift-check tests read process-global env vars.
  use ExUnit.Case, async: false

  alias CouncilHubUi.NodeIdentity

  @autodetected "COUNCIL_NODE_AUTODETECTED"
  @force "COUNCIL_NODE_DRIFT_CHECK"

  setup do
    saved = {System.get_env(@autodetected), System.get_env(@force)}
    System.delete_env(@autodetected)
    System.delete_env(@force)

    on_exit(fn ->
      {auto, force} = saved
      restore_env(@autodetected, auto)
      restore_env(@force, force)
    end)

    :ok
  end

  defp restore_env(var, nil), do: System.delete_env(var)
  defp restore_env(var, value), do: System.put_env(var, value)

  describe "registered_host/0" do
    test "is nil when not distributed" do
      # mix test runs as :nonode@nohost.
      assert NodeIdentity.registered_host() == nil
    end
  end

  describe "current_ip/0" do
    test "returns a binary or nil, never raises" do
      assert is_binary(NodeIdentity.current_ip()) or is_nil(NodeIdentity.current_ip())
    end
  end

  describe "status/0" do
    test "never reports drift when not distributed" do
      assert NodeIdentity.status() == %{
               registered: nil,
               current: nil,
               drifted?: false,
               checkable?: false
             }
    end
  end

  # The pure form takes its three inputs directly, so the drift logic is
  # testable without a distributed node — mix test always runs as
  # :nonode@nohost, which short-circuits status/0 before any comparison.
  describe "status/3" do
    test "reports drift when the addresses differ and the check applies" do
      assert %{drifted?: true, checkable?: true} =
               NodeIdentity.status("192.168.0.5", "192.168.0.9", true)
    end

    test "reports no drift when the addresses match" do
      assert %{drifted?: false, checkable?: true} =
               NodeIdentity.status("192.168.0.5", "192.168.0.5", true)
    end

    test "never claims drift when the comparison doesn't apply to this deployment" do
      # The bridge-networking case: RELEASE_NODE is deliberately the *host's*
      # LAN IP while the container can only measure its own address. Comparing
      # them says nothing about reachability, so a healthy clustered node must
      # not be reported as drifted.
      assert %{drifted?: false, checkable?: false} =
               NodeIdentity.status("192.168.0.5", "172.25.0.2", false)
    end

    test "never claims drift when the current address is unknown" do
      # No route / sandboxed env — an unmeasurable address is not evidence of
      # drift.
      assert %{drifted?: false, checkable?: false} =
               NodeIdentity.status("192.168.0.5", nil, true)
    end

    test "never claims drift when not distributed" do
      assert %{drifted?: false, checkable?: false} =
               NodeIdentity.status(nil, "192.168.0.9", true)
    end

    test "passes both addresses through for display regardless of the verdict" do
      assert %{registered: "council-host", current: "172.25.0.2"} =
               NodeIdentity.status("council-host", "172.25.0.2", false)
    end
  end

  describe "drift_check_enabled?/0" do
    test "is off by default — an explicitly-set RELEASE_NODE is not ours to second-guess" do
      refute NodeIdentity.drift_check_enabled?()
    end

    test "is on when the entrypoint auto-detected the node name" do
      System.put_env(@autodetected, "1")
      assert NodeIdentity.drift_check_enabled?()
    end

    test "can be forced on by the operator" do
      System.put_env(@force, "true")
      assert NodeIdentity.drift_check_enabled?()
    end

    test "an explicit COUNCIL_NODE_DRIFT_CHECK=0 wins over auto-detection" do
      System.put_env(@autodetected, "1")
      System.put_env(@force, "0")
      refute NodeIdentity.drift_check_enabled?()
    end

    test "ignores values that aren't affirmative" do
      System.put_env(@autodetected, "no")
      refute NodeIdentity.drift_check_enabled?()
    end
  end

  describe "static_distribution?/0" do
    test "is false in the test VM, which isn't distributed at all" do
      # Nothing was started, so nothing blocks a rebind — the flag is about
      # distribution started by a boot flag, not about being distributed.
      refute NodeIdentity.static_distribution?()
    end
  end
end
