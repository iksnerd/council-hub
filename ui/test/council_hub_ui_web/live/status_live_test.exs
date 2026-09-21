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

    test "names the stale peer instead of the generic seeds-but-no-peers line", %{conn: conn} do
      finding = %{
        seed: "192.168.0.4",
        host: "192.168.0.4",
        reported_node: "bob@192.168.0.5",
        status: :stale_name,
        message:
          "seed 192.168.0.4 is up but reports node bob@192.168.0.5, an address it does not hold"
      }

      System.put_env("COUNCIL_SEEDS", "192.168.0.4")
      send(CouncilHubUi.ClusterManager, {:seed_status, [finding]})

      on_exit(fn ->
        System.delete_env("COUNCIL_SEEDS")
        send(CouncilHubUi.ClusterManager, {:seed_status, []})
      end)

      _ = CouncilHubUi.ClusterManager.seed_status()

      {:ok, _view, html} = live(conn, "/status")

      assert html =~ "bob@192.168.0.5"
      refute html =~ "Seeds are configured but no peers are connected yet"
    end

    test "keeps env var names intact in a seed finding", %{conn: conn} do
      finding = %{
        seed: "192.168.0.4",
        host: "192.168.0.4",
        reported_node: "bob@192.168.0.4",
        status: :undialable,
        message: "seed 192.168.0.4 is up, but no link formed — check that RELEASE_COOKIE matches"
      }

      System.put_env("COUNCIL_SEEDS", "192.168.0.4")
      send(CouncilHubUi.ClusterManager, {:seed_status, [finding]})

      on_exit(fn ->
        System.delete_env("COUNCIL_SEEDS")
        send(CouncilHubUi.ClusterManager, {:seed_status, []})
      end)

      _ = CouncilHubUi.ClusterManager.seed_status()

      {:ok, _view, html} = live(conn, "/status")

      # The page has its own "no RELEASE_COOKIE" badge, so assert on the
      # finding's own sentence rather than the bare env var name.
      assert html =~ "Seed 192.168.0.4 is up, but no link formed"
      assert html =~ "check that RELEASE_COOKIE matches"
    end

    test "warns when nothing answers on the address this node advertises", %{conn: conn} do
      send(
        CouncilHubUi.ClusterManager,
        {:advertised_status,
         %{
           host: "192.168.0.10",
           port: 4369,
           checkable?: true,
           reachable?: false,
           warning: "nothing answers on 192.168.0.10:4369, the address this node advertises."
         }}
      )

      on_exit(fn ->
        send(
          CouncilHubUi.ClusterManager,
          {:advertised_status,
           %{host: nil, port: nil, checkable?: false, reachable?: nil, warning: nil}}
        )
      end)

      _ = CouncilHubUi.ClusterManager.advertised_status()

      {:ok, _view, html} = live(conn, "/status")

      assert html =~ "Nothing answers on 192.168.0.10:4369"
      refute html =~ "advertises.."
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
