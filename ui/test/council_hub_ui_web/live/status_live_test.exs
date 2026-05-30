defmodule CouncilHubUiWeb.StatusLiveTest do
  use CouncilHubUiWeb.ConnCase, async: true

  import Phoenix.LiveViewTest

  describe "status page" do
    test "renders node, cluster, database, and semantic-search sections", %{conn: conn} do
      {:ok, _view, html} = live(conn, "/status")

      assert html =~ "This node"
      assert html =~ "Cluster peers"
      assert html =~ "Database"
      assert html =~ "Semantic search"
      # node identity is always present (at minimum nonode@nohost in test)
      assert html =~ to_string(Node.self())
    end

    test "is public — no admin token required", %{conn: conn} do
      conn = get(conn, "/status")
      assert html_response(conn, 200) =~ "Status"
    end

    test "semantic search shows 'Not available' when the vec table is absent", %{conn: conn} do
      # The Phoenix test DB has no Go-owned message_vectors table, so coverage
      # is unavailable and the panel says so rather than crashing.
      {:ok, _view, html} = live(conn, "/status")
      assert html =~ "Not available"
    end
  end
end
