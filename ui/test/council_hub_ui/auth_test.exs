defmodule CouncilHubUi.AuthTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.Auth

  @good "test-token-0123456789abcdef"

  describe "validate/1 (the boot check)" do
    test "unset or empty means auth off" do
      assert Auth.validate(nil) == {:ok, nil}
      assert Auth.validate("") == {:ok, nil}
    end

    test "a token of 16 characters or more is accepted" do
      assert Auth.validate(@good) == {:ok, @good}
      assert Auth.validate(String.duplicate("a", 16)) == {:ok, String.duplicate("a", 16)}
    end

    test "a short token is refused, naming the variable and minimum, never the value" do
      short = "short-secret-15c"
      assert String.length(short) == 16
      assert {:ok, _} = Auth.validate(short)

      tiny = "tiny-secret"
      assert {:error, message} = Auth.validate(tiny)
      assert message =~ "COUNCIL_AUTH_TOKEN"
      assert message =~ "16"
      refute message =~ tiny
    end
  end

  describe "status_line/1 and cookie_warning/4" do
    test "status lines" do
      assert {:on, "Authentication: on"} = Auth.status_line(true)

      assert {:off, "Authentication: off — anyone who can reach this port" <> _} =
               Auth.status_line(false)
    end

    test "cookie warning only when auth on, default cookie and clustered" do
      assert Auth.cookie_warning(true, true, "10.0.0.5", []) =~
               "does not protect the cluster ports"

      assert Auth.cookie_warning(true, true, nil, ["a@b"]) =~ "RELEASE_COOKIE"
      assert Auth.cookie_warning(false, true, "10.0.0.5", []) == nil
      assert Auth.cookie_warning(true, false, "10.0.0.5", []) == nil
      assert Auth.cookie_warning(true, true, nil, []) == nil
    end
  end
end
