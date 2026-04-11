defmodule CouncilHubUiWeb.CouncilLiveUiInteractionsTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest
  import CouncilHubUi.CouncilFixtures

  describe "events" do
    test "toggle_system_prompt shows/hides prompt", %{conn: conn} do
      create_room(%{id: "prompt-room", system_prompt: "Be helpful and concise"})

      {:ok, view, html} = live(conn, "/rooms/prompt-room")
      # System prompt should not be visible initially
      refute html =~ "Be helpful and concise"

      # Click toggle
      html = view |> element("button[phx-click='toggle_system_prompt']") |> render_click()
      assert html =~ "Be helpful and concise"
    end

    test "filter_rooms filters sidebar", %{conn: conn} do
      create_room(%{id: "alpha-room", description: "Alpha"})
      create_room(%{id: "beta-room", description: "Beta"})

      {:ok, view, _html} = live(conn, "/")

      html =
        view
        |> element("form[phx-change='filter_rooms']")
        |> render_change(%{query: "alpha"})

      assert html =~ "alpha-room"
      refute html =~ "beta-room"
    end
  end

  describe "filter_rooms/2" do
    test "empty query returns all" do
      rooms = [%{id: "a", description: "Alpha"}, %{id: "b", description: "Beta"}]
      assert CouncilHubUiWeb.CouncilLive.filter_rooms(rooms, "") == rooms
    end

    test "filters by id and description" do
      rooms = [
        %{id: "auth-room", description: "Auth work"},
        %{id: "db-room", description: "DB stuff"}
      ]

      filtered = CouncilHubUiWeb.CouncilLive.filter_rooms(rooms, "auth")
      assert length(filtered) == 1
      assert hd(filtered).id == "auth-room"
    end

    test "case insensitive" do
      rooms = [%{id: "JWT-room", description: "JWT work"}]
      assert length(CouncilHubUiWeb.CouncilLive.filter_rooms(rooms, "jwt")) == 1
    end
  end

  describe "v0.15.0 features" do
    test "critique filter button is present in message type bar", %{conn: conn} do
      create_room(%{id: "critique-filter-room"})
      {:ok, _view, html} = live(conn, "/rooms/critique-filter-room")
      assert html =~ "Critique"
    end

    test "cluster warnings section is absent when no warnings", %{conn: conn} do
      create_room(%{id: "warn-room"})
      {:ok, _view, html} = live(conn, "/rooms/warn-room")
      # When cluster_warnings is empty (default), the amber banner section must not render
      refute html =~ "hero-exclamation-triangle"
    end

    test "room header renders updated_at hook div when present", %{conn: conn} do
      create_room(%{id: "updated-header-room"})
      {:ok, _view, html} = live(conn, "/rooms/updated-header-room")
      # updated_at RelativeTime hook div has id starting with "header-updated-"
      assert html =~ "header-updated-updated-header-room"
    end
  end

  describe "v0.16.0 features" do
    test "compiled badge appears on room card when room has a synthesis message", %{conn: conn} do
      room = create_room(%{id: "synth-badge-room"})

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "Compiled knowledge article",
        message_type: "synthesis"
      })

      {:ok, _view, html} = live(conn, "/")
      # The compiled badge should appear for this room
      assert html =~ "synth-badge-room"
      assert html =~ "title=\"Has synthesis\""
    end

    test "compiled badge absent on room without synthesis messages", %{conn: conn} do
      room = create_room(%{id: "no-synth-room"})

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "Just a thought",
        message_type: "thought"
      })

      {:ok, _view, html} = live(conn, "/")
      assert html =~ "no-synth-room"
      # synthesis badge only appears for synthesized rooms
      refute html =~ "title=\"Has synthesis\""
    end

    test "status is rendered as a dot (no toggle_status button)", %{conn: conn} do
      create_room(%{id: "toggle-status-room", status: "active"})
      {:ok, _view, html} = live(conn, "/")
      refute html =~ "phx-click=\"toggle_status\""
      assert html =~ "toggle-status-room"
    end

    test "synthesis_flags assign is populated on mount for rooms with synthesis", %{conn: conn} do
      room = create_room(%{id: "synth-flags-room"})

      create_message(%{
        room_id: room.id,
        author: "Gemini",
        content: "Knowledge synthesis",
        message_type: "synthesis"
      })

      {:ok, view, _html} = live(conn, "/")
      # The MapSet is in assigns — test indirectly via compiled badge in rendered HTML
      assert render(view) =~ "synth-flags-room"
      assert render(view) =~ "title=\"Has synthesis\""
    end

    test "filter toggle button is present in message search bar", %{conn: conn} do
      create_room(%{id: "filter-btn-room"})
      {:ok, _view, html} = live(conn, "/rooms/filter-btn-room")
      assert html =~ "toggle_search_filters"
      assert html =~ "hero-adjustments-horizontal"
    end

    test "toggle_search_filters shows and hides the filter panel", %{conn: conn} do
      create_room(%{id: "filter-panel-room"})
      {:ok, view, html} = live(conn, "/rooms/filter-panel-room")
      # Panel hidden by default
      refute html =~ "apply_search_filters"

      view |> element("button[phx-click='toggle_search_filters']") |> render_click()
      assert render(view) =~ "apply_search_filters"
      assert render(view) =~ ~s(name="author")
      assert render(view) =~ ~s(name="since")
      assert render(view) =~ ~s(name="until")
    end

    test "apply_search_filters by author returns filtered messages", %{conn: conn} do
      room = create_room(%{id: "filter-author-room"})
      create_message(%{room_id: room.id, author: "Claude", content: "From Claude"})
      create_message(%{room_id: room.id, author: "Gemini", content: "From Gemini"})

      {:ok, view, _html} = live(conn, "/rooms/filter-author-room")
      view |> element("button[phx-click='toggle_search_filters']") |> render_click()

      html =
        view
        |> form("form[phx-change='apply_search_filters']", %{
          "author" => "Claude",
          "since" => "",
          "until" => ""
        })
        |> render_change()

      assert html =~ "From Claude"
      refute html =~ "From Gemini"
    end
  end

  describe "MCP-dependent room actions" do
    test "toggle_status does not crash for all status transitions", %{conn: conn} do
      room = create_room(%{id: "toggle-status-event-room"})
      {:ok, view, _html} = live(conn, "/rooms/toggle-status-event-room")

      # active → paused (McpClient will fail but event must not crash)
      render_click(view, "toggle_status", %{"room-id" => room.id, "status" => "active"})
      assert render(view) =~ "toggle-status-event-room"

      # paused → resolved
      render_click(view, "toggle_status", %{"room-id" => room.id, "status" => "paused"})
      assert render(view) =~ "toggle-status-event-room"

      # anything else → active
      render_click(view, "toggle_status", %{"room-id" => room.id, "status" => "resolved"})
      assert render(view) =~ "toggle-status-event-room"
    end

    test "archive_room sets a flash message (info on success, error when MCP unreachable)", %{
      conn: conn
    } do
      create_room(%{id: "archive-event-room", status: "resolved"})
      {:ok, view, _html} = live(conn, "/rooms/archive-event-room")

      render_click(view, "archive_room", %{"room-id" => "archive-event-room"})

      # Either success (info flash) or error (error flash) — either way, flash is set
      flash = :sys.get_state(view.pid).socket.assigns.flash
      assert map_size(flash) > 0
    end

    test "check_room_health sets a flash message (info on success, error when MCP unreachable)",
         %{conn: conn} do
      create_room(%{id: "health-event-room"})
      {:ok, view, _html} = live(conn, "/rooms/health-event-room")

      render_click(view, "check_room_health", %{"room-id" => "health-event-room"})

      flash = :sys.get_state(view.pid).socket.assigns.flash
      assert map_size(flash) > 0
    end

    test "save_tags closes the editor regardless of MCP server state", %{conn: conn} do
      create_room(%{id: "save-tags-event-room", tags: "go"})
      {:ok, view, _html} = live(conn, "/rooms/save-tags-event-room")

      view |> element("button[phx-click='edit_tags']") |> render_click()
      render_click(view, "save_tags", %{"tags" => "go,elixir"})

      # editing_tags is false in both success and error paths
      assert :sys.get_state(view.pid).socket.assigns.editing_tags == false
    end

    test "view_archive sets active_archive with error content when MCP server is unreachable", %{
      conn: conn
    } do
      create_room(%{id: "view-archive-event-room"})
      {:ok, view, _html} = live(conn, "/rooms/view-archive-event-room")

      render_click(view, "view_archive", %{"room-id" => "view-archive-event-room"})

      assigns = :sys.get_state(view.pid).socket.assigns
      assert assigns.active_archive != nil
      assert assigns.active_archive.room_id == "view-archive-event-room"
      assert assigns.active_archive.content =~ "Failed to load archive"
    end
  end

  describe "interactive room actions" do
    test "edit_tags event shows tag input form", %{conn: conn} do
      create_room(%{id: "edit-tags-room", tags: "go,elixir"})

      {:ok, view, _html} = live(conn, "/rooms/edit-tags-room")
      view |> element("button[phx-click='edit_tags']") |> render_click()

      html = render(view)
      assert html =~ ~s(name="tags")
      assert html =~ "save"
      assert html =~ "cancel"
    end

    test "cancel_edit_tags hides tag input", %{conn: conn} do
      create_room(%{id: "cancel-tags-room", tags: "go"})

      {:ok, view, _html} = live(conn, "/rooms/cancel-tags-room")
      view |> element("button[phx-click='edit_tags']") |> render_click()
      view |> element("button[phx-click='cancel_edit_tags']") |> render_click()

      html = render(view)
      refute html =~ ~s(name="tags")
    end

    test "quick archive button appears for resolved rooms", %{conn: conn} do
      create_room(%{id: "resolved-room", status: "resolved"})

      {:ok, _view, html} = live(conn, "/rooms/resolved-room")
      assert html =~ "archive"
      assert html =~ "phx-click=\"archive_room\""
    end

    test "quick archive button absent for active rooms", %{conn: conn} do
      create_room(%{id: "active-room-noarchive", status: "active"})

      {:ok, _view, html} = live(conn, "/rooms/active-room-noarchive")
      refute html =~ "phx-click=\"archive_room\""
    end

    test "lint button is present in room header", %{conn: conn} do
      create_room(%{id: "lint-btn-room"})

      {:ok, _view, html} = live(conn, "/rooms/lint-btn-room")
      assert html =~ "phx-click=\"check_room_health\""
    end

    test "close_archive event clears active_archive", %{conn: conn} do
      create_room(%{id: "close-archive-room"})

      {:ok, view, _html} = live(conn, "/rooms/close-archive-room")
      # Trigger close_archive (sets active_archive: nil)
      render_click(view, "close_archive", %{})

      html = render(view)
      # archive modal should not be present when active_archive is nil
      refute html =~ "— archive"
    end
  end
end
