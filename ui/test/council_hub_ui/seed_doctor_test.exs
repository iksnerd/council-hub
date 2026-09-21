defmodule CouncilHubUi.SeedDoctorTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.SeedDoctor

  describe "check/3 — the seed host is up but the name it reports is not" do
    test "flags a seed that answers /health under a node name for a different address" do
      probe = fn "192.168.0.4" -> {:ok, "bob@192.168.0.5"} end

      assert [finding] = SeedDoctor.check("192.168.0.4", [], probe)
      assert finding.status == :stale_name
      assert finding.host == "192.168.0.4"
      assert finding.reported_node == "bob@192.168.0.5"
      assert finding.message =~ "192.168.0.4"
      assert finding.message =~ "bob@192.168.0.5"
    end
  end

  describe "check/3 — the other two outcomes" do
    test "reports a seed whose reported name agrees with its address but still has no link" do
      probe = fn "192.168.0.4" -> {:ok, "bob@192.168.0.4"} end

      assert [finding] = SeedDoctor.check("192.168.0.4", [], probe)
      assert finding.status == :undialable
      assert finding.message =~ "RELEASE_COOKIE"
    end

    test "does not claim a seed is down when it answered without a node name" do
      # e.g. the peer runs COUNCIL_UI=off, so /health carries no cluster_nodes.
      probe = fn "192.168.0.4" -> {:error, :no_node_in_health} end

      assert [finding] = SeedDoctor.check("192.168.0.4", [], probe)
      assert finding.status == :no_node_name
      refute finding.message =~ "not answering"
    end

    test "reports a seed that does not answer at all" do
      probe = fn "192.168.0.4" -> {:error, :timeout} end

      assert [finding] = SeedDoctor.check("192.168.0.4", [], probe)
      assert finding.status == :unreachable
      assert finding.reported_node == nil
    end
  end

  describe "check/3 — when there is nothing to diagnose" do
    test "says nothing while a peer is connected" do
      probe = fn _ -> flunk("must not probe while the cluster is up") end

      assert SeedDoctor.check("192.168.0.4", [:"bob@192.168.0.4"], probe) == []
    end

    test "says nothing when no seeds are configured" do
      probe = fn _ -> flunk("must not probe without seeds") end

      assert SeedDoctor.check(nil, [], probe) == []
      assert SeedDoctor.check("", [], probe) == []
    end

    test "never calls a loopback seed stale — the two addresses aren't comparable" do
      # A container probed over loopback legitimately advertises its LAN IP, so
      # comparing the two says nothing. Same reasoning as NodeIdentity's
      # checkable? guard: a verdict that can't be trusted is worse than silence.
      probe = fn _ -> {:ok, "council_hub@192.168.0.6"} end

      assert SeedDoctor.check("127.0.0.1", [], probe) == []
      assert SeedDoctor.check("localhost", [], probe) == []
      assert SeedDoctor.check("::1", [], probe) == []
    end

    test "probes every configured seed, and takes the host from a node@host entry" do
      probe = fn
        "192.168.0.4" -> {:ok, "bob@192.168.0.5"}
        "10.0.0.9" -> {:error, :econnrefused}
      end

      assert [stale, down] = SeedDoctor.check("192.168.0.4, council_hub@10.0.0.9", [], probe)
      assert stale.status == :stale_name
      assert down.status == :unreachable
      assert down.host == "10.0.0.9"
    end
  end

  describe "probe_url/1" do
    test "targets the Go server's health endpoint on the default port" do
      assert SeedDoctor.probe_url("192.168.0.4") == "http://192.168.0.4:3001/health"
    end

    test "honors COUNCIL_PEER_MCP_PORT" do
      System.put_env("COUNCIL_PEER_MCP_PORT", "3999")
      on_exit(fn -> System.delete_env("COUNCIL_PEER_MCP_PORT") end)

      assert SeedDoctor.probe_url("192.168.0.4") == "http://192.168.0.4:3999/health"
    end
  end

  describe "warning/1" do
    test "is nil when nothing was found" do
      assert SeedDoctor.warning([]) == nil
    end

    test "leads with the stale name, which is the peer-side fault a local operator can't see" do
      findings = [
        %{status: :unreachable, message: "seed 10.0.0.9 is not answering"},
        %{status: :stale_name, message: "seed 192.168.0.4 is up but reports X"}
      ]

      warning = SeedDoctor.warning(findings)
      assert warning =~ "seed 192.168.0.4 is up but reports X"
      assert warning =~ "seed 10.0.0.9 is not answering"
      assert String.starts_with?(warning, "no cluster peers connected")
    end
  end
end
