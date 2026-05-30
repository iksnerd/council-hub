defmodule CouncilHubUiWeb.SettingsLiveTest do
  # async: false — these tests mutate the global COUNCIL_CLUSTER_ADMIN_TOKEN env var.
  use CouncilHubUiWeb.ConnCase, async: false

  @token "test-admin-token"

  setup do
    System.put_env("COUNCIL_CLUSTER_ADMIN_TOKEN", @token)
    on_exit(fn -> System.delete_env("COUNCIL_CLUSTER_ADMIN_TOKEN") end)
    :ok
  end

  defp admin_conn(conn), do: Plug.Test.init_test_session(conn, %{cluster_admin: true})

  describe "gating" do
    test "404 when no admin token is configured on the server", %{conn: conn} do
      System.delete_env("COUNCIL_CLUSTER_ADMIN_TOKEN")
      conn = get(conn, "/settings")
      assert conn.status == 404
    end

    test "403 without a session and without the token param", %{conn: conn} do
      conn = get(conn, "/settings")
      assert conn.status == 403
    end

    test "correct ?token= sets the session and redirects to the clean URL", %{conn: conn} do
      conn = get(conn, "/settings?token=#{@token}")
      assert redirected_to(conn) == "/settings"
      assert get_session(conn, :cluster_admin) == true
    end

    test "wrong ?token= is rejected", %{conn: conn} do
      conn = get(conn, "/settings?token=nope")
      assert conn.status == 403
    end
  end

  describe "mount (authenticated)" do
    test "renders this node's identity and the connect form", %{conn: conn} do
      {:ok, _view, html} = live(admin_conn(conn), "/settings")

      assert html =~ "Cluster Settings"
      assert html =~ to_string(Node.self())
      assert html =~ "Connect a peer"
      assert html =~ "council_hub@100.x.y.z"
    end
  end

  describe "connect event" do
    test "shows an error flash for a malformed node name", %{conn: conn} do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")

      html =
        view
        |> form("form", %{"node" => "garbage"})
        |> render_submit()

      assert html =~ "invalid node name"
    end
  end
end
