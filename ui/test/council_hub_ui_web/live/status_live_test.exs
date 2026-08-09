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

    test "no identity-drift badge or warning when not distributed", %{conn: conn} do
      # mix test runs as :nonode@nohost, so NodeIdentity.status/0 never
      # reports drift — the badge/doctor line must not render.
      {:ok, _view, html} = live(conn, "/status")
      refute html =~ "identity drifted"
      refute html =~ "Node identity stale"
    end

    test "regenerate_embeddings shows an error flash when the MCP server is unreachable", %{
      conn: conn
    } do
      # The test env has no live Go MCP server on the configured port, so
      # McpClient.backfill_embeddings/1 hits its error path regardless of the
      # "Backfill missing"/"Full re-embed" buttons being in the DOM (they're
      # hidden here anyway — no message_vectors table, see the test above).
      {:ok, view, _html} = live(conn, "/status")
      html = render_click(view, "regenerate_embeddings", %{"full" => "false"})
      assert html =~ "Regenerate failed"
    end
  end
end
