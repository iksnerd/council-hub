defmodule CouncilHubUi.NodeIdentityTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.NodeIdentity

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
      assert NodeIdentity.status() == %{registered: nil, current: nil, drifted?: false}
    end
  end
end
