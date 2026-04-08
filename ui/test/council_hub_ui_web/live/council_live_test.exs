defmodule CouncilHubUiWeb.CouncilLiveTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest
  import CouncilHubUi.CouncilFixtures

  describe "mount" do
    test "renders empty state with no rooms", %{conn: conn} do
      {:ok, _view, html} = live(conn, "/")
      assert html =~ "Council Hub"
    end

    test "renders rooms in sidebar", %{conn: conn} do
      create_room(%{id: "live-room-1", description: "First room", project: "proj-a"})
      create_room(%{id: "live-room-2", description: "Second room", project: "proj-b"})

      {:ok, _view, html} = live(conn, "/")
      assert html =~ "live-room-1"
      assert html =~ "live-room-2"
    end
  end

  describe "room navigation" do
    test "selecting a room shows messages", %{conn: conn} do
      room = create_room(%{id: "nav-room", description: "Navigation test"})
      create_message(%{room_id: room.id, author: "Claude", content: "Hello from test"})

      {:ok, view, _html} = live(conn, "/")
      view |> element("a[href='/rooms/nav-room']") |> render_click()

      # After navigation, should show room content
      html = render(view)
      assert html =~ "nav-room"
      assert html =~ "Hello from test"
    end

    test "direct URL to room works", %{conn: conn} do
      room = create_room(%{id: "direct-room", description: "Direct access"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Direct message"})

      {:ok, _view, html} = live(conn, "/rooms/direct-room")
      assert html =~ "direct-room"
      assert html =~ "Direct message"
    end

    test "nonexistent room redirects with flash", %{conn: conn} do
      assert {:error, {:live_redirect, %{to: "/", flash: %{"error" => flash}}}} =
               live(conn, "/rooms/nonexistent")

      assert flash =~ "not found"
    end
  end

  describe "room metadata display" do
    test "shows project and tech stack in header", %{conn: conn} do
      create_room(%{
        id: "meta-room",
        description: "Metadata test",
        project: "my-project",
        tech_stack: "Elixir, Go"
      })

      {:ok, _view, html} = live(conn, "/rooms/meta-room")
      assert html =~ "my-project"
      assert html =~ "Elixir, Go"
    end

    test "shows related rooms in header", %{conn: conn} do
      create_room(%{
        id: "linked-room",
        description: "Linked test",
        related_rooms: "room-a,room-b"
      })

      {:ok, _view, html} = live(conn, "/rooms/linked-room")
      assert html =~ "room-a"
      assert html =~ "room-b"
    end

    test "shows tags in header", %{conn: conn} do
      create_room(%{id: "tagged-room", description: "Tagged", tags: "auth,security"})

      {:ok, _view, html} = live(conn, "/rooms/tagged-room")
      assert html =~ "auth"
      assert html =~ "security"
    end
  end

  describe "message display" do
    test "shows message types", %{conn: conn} do
      room = create_room(%{id: "type-room"})

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "A thought",
        message_type: "thought"
      })

      create_message(%{
        room_id: room.id,
        author: "Gemini",
        content: "A critique",
        message_type: "critique"
      })

      {:ok, _view, html} = live(conn, "/rooms/type-room")
      assert html =~ "thought"
      assert html =~ "critique"
    end

    test "shows reply_to badge", %{conn: conn} do
      room = create_room(%{id: "reply-display"})
      m1 = create_message(%{room_id: room.id, author: "Claude", content: "Original"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Reply", reply_to: m1.id})

      {:ok, _view, html} = live(conn, "/rooms/reply-display")
      assert html =~ "re: ##{String.slice(m1.id, 0, 8)}"
    end

    test "shows summary blocks", %{conn: conn} do
      room = create_room(%{id: "summary-room"})

      create_message(%{
        room_id: room.id,
        author: "System",
        content: "Summary content",
        is_summary: true
      })

      {:ok, _view, html} = live(conn, "/rooms/summary-room")
      assert html =~ "Summary"
    end
  end

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

  describe "toggle_summary" do
    test "toggles summary collapsed state", %{conn: conn} do
      room = create_room(%{id: "toggle-room"})

      create_message(%{
        room_id: room.id,
        author: "System",
        content: "Summary text",
        is_summary: true
      })

      {:ok, view, _html} = live(conn, "/rooms/toggle-room")

      # Click collapse toggle
      html = view |> element("button[phx-click='toggle_summary']") |> render_click()
      assert html =~ "expand"

      # Click again to expand
      html = view |> element("button[phx-click='toggle_summary']") |> render_click()
      assert html =~ "collapse"
    end
  end

  describe "polling" do
    test "new messages appear after poll", %{conn: conn} do
      room = create_room(%{id: "poll-room"})
      create_message(%{room_id: room.id, author: "Claude", content: "Initial message"})

      {:ok, view, html} = live(conn, "/rooms/poll-room")
      assert html =~ "Initial message"

      # Add a new message and trigger poll
      create_message(%{room_id: room.id, author: "Gemini", content: "Polled message"})
      send(view.pid, :poll_messages)

      html = render(view)
      assert html =~ "Polled message"
    end

    test "poll_messages with no active room is noop", %{conn: conn} do
      {:ok, view, _html} = live(conn, "/")
      send(view.pid, :poll_messages)
      # Should not crash
      assert render(view) =~ "Council Hub"
    end

    test "poll_messages with no new messages is noop", %{conn: conn} do
      room = create_room(%{id: "poll-noop"})
      create_message(%{room_id: room.id, author: "Claude", content: "Only message"})

      {:ok, view, _html} = live(conn, "/rooms/poll-noop")
      send(view.pid, :poll_messages)
      assert render(view) =~ "Only message"
    end

    test "poll_rooms updates room list", %{conn: conn} do
      create_room(%{id: "poll-rooms-1", description: "First"})
      {:ok, view, _html} = live(conn, "/")

      # Add another room and trigger poll
      create_room(%{id: "poll-rooms-2", description: "Second"})
      send(view.pid, :poll_rooms)

      html = render(view)
      assert html =~ "poll-rooms-2"
    end

    test "poll_rooms skips when nothing changed", %{conn: conn} do
      create_room(%{id: "unchanged-room"})
      {:ok, view, _html} = live(conn, "/")

      # Trigger poll twice — second should skip (same latest_room_update)
      send(view.pid, :poll_rooms)
      send(view.pid, :poll_rooms)
      assert render(view) =~ "unchanged-room"
    end

    test "poll_rooms updates active room status", %{conn: conn} do
      room = create_room(%{id: "status-poll", status: "active"})
      create_message(%{room_id: room.id, author: "Claude", content: "msg"})

      {:ok, view, html} = live(conn, "/rooms/status-poll")
      assert html =~ "active"

      # Update room status directly in DB
      import Ecto.Query

      CouncilHubUi.Repo.update_all(
        from(r in CouncilHubUi.Council.Room, where: r.id == "status-poll"),
        set: [status: "resolved", updated_at: NaiveDateTime.add(NaiveDateTime.utc_now(), 1)]
      )

      send(view.pid, :poll_rooms)
      html = render(view)
      assert html =~ "resolved"
    end
  end

  describe "nil project handling" do
    test "rooms with nil project go to ungrouped" do
      rooms = [%{id: "x", project: nil, description: "Nil project", status: "active"}]
      grouped = CouncilHubUiWeb.CouncilLive.group_rooms_by_project(rooms)
      assert [{"ungrouped", _}] = grouped
    end
  end

  describe "group_rooms_by_project/1" do
    test "groups rooms by project" do
      rooms = [
        %{id: "a", project: "proj-1", description: "", status: "active"},
        %{id: "b", project: "proj-2", description: "", status: "active"},
        %{id: "c", project: "proj-1", description: "", status: "active"}
      ]

      grouped = CouncilHubUiWeb.CouncilLive.group_rooms_by_project(rooms)
      projects = Enum.map(grouped, fn {p, _} -> p end)
      assert "proj-1" in projects
      assert "proj-2" in projects
    end

    test "ungrouped rooms go to 'ungrouped'" do
      rooms = [%{id: "x", project: "", description: "", status: "active"}]
      grouped = CouncilHubUiWeb.CouncilLive.group_rooms_by_project(rooms)
      assert [{"ungrouped", _}] = grouped
    end
  end

  describe "react event" do
    test "react event does not crash when MCP server is unreachable", %{conn: conn} do
      room = create_room(%{id: "react-room"})
      create_message(%{room_id: room.id, author: "Claude", content: "React to this"})

      {:ok, view, _html} = live(conn, "/rooms/react-room")

      # McpClient will fail to connect (no server running in test) but must not crash the LiveView
      view
      |> render_hook("react", %{"message-id" => "some-msg-id", "emoji" => "👍"})

      assert render(view) =~ "react-room"
    end
  end

  describe "latest_ids assign" do
    test "latest_ids is populated on mount when messages exist", %{conn: conn} do
      room = create_room(%{id: "lid-room"})
      msg = create_message(%{room_id: room.id, author: "Claude", content: "msg"})

      {:ok, view, _html} = live(conn, "/")
      latest_ids = :sys.get_state(view.pid).socket.assigns.latest_ids
      assert Map.get(latest_ids, room.id) == msg.id
    end

    test "latest_ids is empty map when no messages", %{conn: conn} do
      {:ok, view, _html} = live(conn, "/")
      assert :sys.get_state(view.pid).socket.assigns.latest_ids == %{}
    end
  end

  describe "filter_type event" do
    test "filters messages by type", %{conn: conn} do
      room = create_room(%{id: "ft-room"})

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "A decision",
        message_type: "decision"
      })

      create_message(%{
        room_id: room.id,
        author: "Gemini",
        content: "A thought",
        message_type: "thought"
      })

      {:ok, view, _html} = live(conn, "/rooms/ft-room")

      html =
        view
        |> element("button[phx-click='filter_type'][phx-value-type='decision']")
        |> render_click()

      assert html =~ "A decision"
      refute html =~ "A thought"
    end

    test "filters messages by synthesis type", %{conn: conn} do
      room = create_room(%{id: "ft-synth-room"})

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "A synthesis article",
        message_type: "synthesis"
      })

      create_message(%{
        room_id: room.id,
        author: "Gemini",
        content: "A thought",
        message_type: "thought"
      })

      {:ok, view, _html} = live(conn, "/rooms/ft-synth-room")

      html =
        view
        |> element("button[phx-click='filter_type'][phx-value-type='synthesis']")
        |> render_click()

      assert html =~ "A synthesis article"
      refute html =~ "A thought"
    end

    test "all filter shows all messages", %{conn: conn} do
      room = create_room(%{id: "ft-all-room"})

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "A decision",
        message_type: "decision"
      })

      create_message(%{
        room_id: room.id,
        author: "Gemini",
        content: "A thought",
        message_type: "thought"
      })

      {:ok, view, _html} = live(conn, "/rooms/ft-all-room")

      html =
        view
        |> element("button[phx-click='filter_type'][phx-value-type='all']")
        |> render_click()

      assert html =~ "A decision"
      assert html =~ "A thought"
    end
  end

  describe "search_messages event" do
    test "search returns matching messages", %{conn: conn} do
      room = create_room(%{id: "sm-room"})
      create_message(%{room_id: room.id, author: "Claude", content: "Authentication fix"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Unrelated post"})

      {:ok, view, _html} = live(conn, "/rooms/sm-room")

      html =
        view
        |> element("form[phx-change='search_messages']")
        |> render_change(%{query: "authentication"})

      assert html =~ "Authentication fix"
      refute html =~ "Unrelated post"
    end

    test "clearing search restores full message list", %{conn: conn} do
      room = create_room(%{id: "sm-clear-room"})
      create_message(%{room_id: room.id, author: "Claude", content: "Authentication fix"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Unrelated post"})

      {:ok, view, _html} = live(conn, "/rooms/sm-clear-room")

      # Search first
      view
      |> element("form[phx-change='search_messages']")
      |> render_change(%{query: "authentication"})

      # Then clear
      html =
        view
        |> element("form[phx-change='search_messages']")
        |> render_change(%{query: ""})

      assert html =~ "Authentication fix"
      assert html =~ "Unrelated post"
    end

    test "poll does not override search results", %{conn: conn} do
      room = create_room(%{id: "sm-poll-room"})
      create_message(%{room_id: room.id, author: "Claude", content: "Auth fix"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Unrelated"})

      {:ok, view, _html} = live(conn, "/rooms/sm-poll-room")

      view
      |> element("form[phx-change='search_messages']")
      |> render_change(%{query: "auth"})

      # Poll arrives — should update last_msg_id but not change stream
      create_message(%{room_id: room.id, author: "Claude", content: "New message"})
      send(view.pid, :poll_messages)

      html = render(view)
      assert html =~ "Auth fix"
      # "Unrelated" should NOT be back in the stream
      refute html =~ "Unrelated"
    end
  end

  describe "sidebar project accordion" do
    test "rooms grouped by project render in sidebar", %{conn: conn} do
      create_room(%{id: "acc-room-a", project: "proj-alpha"})
      create_room(%{id: "acc-room-b", project: "proj-beta"})

      {:ok, _view, html} = live(conn, "/")
      assert html =~ "proj-alpha"
      assert html =~ "proj-beta"
      assert html =~ "acc-room-a"
      assert html =~ "acc-room-b"
    end

    test "active room's project group is open", %{conn: conn} do
      create_room(%{id: "acc-active", project: "proj-active"})
      create_message(%{room_id: "acc-active", author: "Claude", content: "msg"})

      {:ok, _view, html} = live(conn, "/rooms/acc-active")
      # The details element for the active project should have open attribute
      assert html =~ "acc-active"
    end
  end

  describe "toggle_cluster_wide" do
    test "defaults to local mode", %{conn: conn} do
      {:ok, view, html} = live(conn, "/")
      assert html =~ "○ local"
      assert :sys.get_state(view.pid).socket.assigns.cluster_wide == false
    end

    test "toggles to all-nodes mode and back", %{conn: conn} do
      create_room(%{id: "toggle-cw-room"})
      {:ok, view, html} = live(conn, "/")
      assert html =~ "○ local"

      html = view |> element("button[phx-click='toggle_cluster_wide']") |> render_click()
      assert html =~ "● all"
      assert :sys.get_state(view.pid).socket.assigns.cluster_wide == true

      html = view |> element("button[phx-click='toggle_cluster_wide']") |> render_click()
      assert html =~ "○ local"
      assert :sys.get_state(view.pid).socket.assigns.cluster_wide == false
    end

    test "poll_rooms when cluster_wide always reloads", %{conn: conn} do
      create_room(%{id: "cw-poll-room"})
      {:ok, view, _html} = live(conn, "/")

      # Enable cluster_wide
      view |> element("button[phx-click='toggle_cluster_wide']") |> render_click()

      # Add a new room and trigger poll — should show up even without DB change sentinel
      create_room(%{id: "cw-poll-room-2"})
      send(view.pid, :poll_rooms)

      assert render(view) =~ "cw-poll-room-2"
    end
  end

  describe "cluster tracking" do
    test "assigns nodes on mount", %{conn: conn} do
      {:ok, view, _html} = live(conn, "/")
      assert %{nodes: nodes} = :sys.get_state(view.pid).socket.assigns
      assert Node.self() in nodes
    end

    test "poll_cluster updates node list", %{conn: conn} do
      {:ok, view, _html} = live(conn, "/")

      # Initially should just be local node
      assert :sys.get_state(view.pid).socket.assigns.nodes == [Node.self()]

      # Trigger poll
      send(view.pid, :poll_cluster)

      # Should still be at least local node, and wouldn't crash
      assert Node.self() in :sys.get_state(view.pid).socket.assigns.nodes
    end

    test "renders cluster nodes in sidebar", %{conn: conn} do
      {:ok, _view, html} = live(conn, "/")
      assert html =~ "Cluster Nodes"
      assert html =~ "#{Node.self()}"
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
      # The compiled badge (book-open icon) should appear for this room
      assert html =~ "synth-badge-room"
      assert html =~ "hero-book-open"
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
      # hero-book-open only appears for synthesized rooms
      refute html =~ "hero-book-open"
    end

    test "status badge is a clickable button with toggle_status handler", %{conn: conn} do
      create_room(%{id: "toggle-status-room", status: "active"})
      {:ok, _view, html} = live(conn, "/")
      assert html =~ "phx-click=\"toggle_status\""
      assert html =~ "phx-value-room-id=\"toggle-status-room\""
    end

    test "toggle_status event does not crash when MCP server is unreachable", %{conn: conn} do
      create_room(%{id: "toggle-crash-room", status: "active"})
      {:ok, view, _html} = live(conn, "/")
      # McpClient will fail to connect but must not crash the LiveView
      view
      |> render_hook("toggle_status", %{"room-id" => "toggle-crash-room", "status" => "active"})

      assert render(view) =~ "toggle-crash-room"
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
      assert render(view) =~ "hero-book-open"
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
  end
end
