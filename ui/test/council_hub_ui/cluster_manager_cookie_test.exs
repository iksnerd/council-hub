defmodule CouncilHubUi.ClusterManagerCookieTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.ClusterManager

  test "a recorded cookie rejection turns an undialable seed into cookie_mismatch" do
    path = Path.join(System.tmp_dir!(), "cm_cookie_#{System.unique_integer([:positive])}.json")
    on_exit(fn -> File.rm(path) end)

    {:ok, pid} =
      GenServer.start_link(ClusterManager,
        path: path,
        reconnect_interval: 3_600_000,
        seeds: "10.0.0.4",
        seed_probe: fn "10.0.0.4" -> {:ok, "bob@10.0.0.4"} end,
        seed_probe_interval: 0
      )

    send(pid, {:cookie_rejected, "bob@10.0.0.4"})
    send(pid, :reconnect)

    assert %{findings: [%{status: :cookie_mismatch, host: "10.0.0.4"}]} =
             wait_for_findings(pid)
  end

  # The probe runs in a Task and reports back with a message, so the result lands
  # at an unknown moment. A fixed sleep passed alone and failed under a loaded
  # full suite; poll with a deadline instead.
  defp wait_for_findings(pid, remaining_ms \\ 5_000) do
    status = ClusterManager.seed_status(pid)

    if status.findings != [] or remaining_ms <= 0 do
      status
    else
      Process.sleep(20)
      wait_for_findings(pid, remaining_ms - 20)
    end
  end
end
