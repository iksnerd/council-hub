defmodule CouncilHubUi.ClusterCookieTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.ClusterCookie

  describe "default?/1" do
    test "is true only for the published default cookie" do
      assert ClusterCookie.default?(:council)
      refute ClusterCookie.default?(:some_other_cookie)
      # A node that is not distributed has no cookie at all.
      refute ClusterCookie.default?(:nocookie)
    end
  end

  describe "warning/3" do
    test "stays silent for a single node on the default cookie" do
      assert ClusterCookie.warning(true, "", []) == nil
      assert ClusterCookie.warning(true, nil, []) == nil
    end

    test "stays silent when the cookie is not the default" do
      assert ClusterCookie.warning(false, "10.0.0.4", ["bob@10.0.0.4"]) == nil
    end

    test "warns when seeds are configured" do
      assert warning = ClusterCookie.warning(true, "10.0.0.4", [])
      assert warning =~ "default cookie, which is public"
      assert warning =~ "RELEASE_COOKIE"
      assert warning =~ "every node"
    end

    test "warns when a peer is connected" do
      assert ClusterCookie.warning(true, "", ["bob@10.0.0.4"]) =~
               "default cookie, which is public"
    end
  end
end
