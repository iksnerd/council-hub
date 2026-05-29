defmodule CouncilHubUiWeb.SettingsLiveTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest

  describe "mount" do
    test "renders this node's identity and the connect form", %{conn: conn} do
      {:ok, _view, html} = live(conn, "/settings")

      assert html =~ "Cluster Settings"
      assert html =~ to_string(Node.self())
      assert html =~ "Connect a peer"
      assert html =~ "council_hub@100.x.y.z"
    end
  end

  describe "connect event" do
    test "shows an error flash for a malformed node name", %{conn: conn} do
      {:ok, view, _html} = live(conn, "/settings")

      html =
        view
        |> form("form", %{"node" => "garbage"})
        |> render_submit()

      assert html =~ "invalid node name"
    end
  end
end
