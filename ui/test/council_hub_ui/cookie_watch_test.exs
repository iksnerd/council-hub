defmodule CouncilHubUi.CookieWatchTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.{CookieWatch, SeedDoctor}

  # The shape OTP's dist_util produces via error_logger:error_msg/2 (OTP 29).
  defp rejection_event(node) do
    %{
      level: :error,
      meta: %{},
      msg:
        {:report,
         %{
           label: {:error_logger, :error_msg},
           format: ~c"** Connection attempt from node ~w rejected. Invalid challenge reply. **~n",
           args: [node]
         }}
    }
  end

  describe "rejected_node/1" do
    test "extracts the dialing node from a bad-cookie rejection" do
      assert CookieWatch.rejected_node(rejection_event(:"bob@10.0.0.4")) == {:ok, "bob@10.0.0.4"}
    end

    test "also reads the classic {format, args} message shape" do
      event = %{
        level: :error,
        meta: %{},
        msg:
          {~c"** Connection attempt from node ~w rejected. Invalid challenge reply. **~n",
           [:"bob@10.0.0.4"]}
      }

      assert CookieWatch.rejected_node(event) == {:ok, "bob@10.0.0.4"}
    end

    test "ignores every other log event" do
      assert CookieWatch.rejected_node(%{level: :error, meta: %{}, msg: {:string, "boom"}}) ==
               :ignore

      other =
        put_in(rejection_event(:"bob@10.0.0.4").msg, {:report, %{format: ~c"x ~w", args: [1]}})

      assert CookieWatch.rejected_node(other) == :ignore
    end
  end

  describe "SeedDoctor.check/4 with rejected hosts" do
    test "reports cookie_mismatch for a seed whose host was rejected" do
      probe = fn "10.0.0.4" -> {:ok, "bob@10.0.0.4"} end

      assert [finding] = SeedDoctor.check("10.0.0.4", [], probe, ["10.0.0.4"])
      assert finding.status == :cookie_mismatch
      assert finding.message =~ "RELEASE_COOKIE"
      assert finding.message =~ "10.0.0.4"
      refute finding.message =~ "ports"
    end

    test "also upgrades a seed whose HTTP port is closed, since the dist link was reached" do
      probe = fn "10.0.0.4" -> {:error, :econnrefused} end
      assert [%{status: :cookie_mismatch}] = SeedDoctor.check("10.0.0.4", [], probe, ["10.0.0.4"])
    end

    test "leaves a stale name alone and other hosts alone" do
      stale = fn "10.0.0.4" -> {:ok, "bob@10.0.0.5"} end
      assert [%{status: :stale_name}] = SeedDoctor.check("10.0.0.4", [], stale, ["10.0.0.4"])

      probe = fn "10.0.0.4" -> {:ok, "bob@10.0.0.4"} end
      assert [%{status: :undialable}] = SeedDoctor.check("10.0.0.4", [], probe, ["10.0.0.9"])
    end

    test "warning/1 lists cookie_mismatch first" do
      findings = [
        %{status: :undialable, message: "undialable one"},
        %{status: :cookie_mismatch, message: "cookie one"}
      ]

      assert SeedDoctor.warning(findings) =~ ~r/cookie one.*undialable one/
    end
  end
end
