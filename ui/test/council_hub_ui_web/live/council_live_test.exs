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
      create_message(%{room_id: room.id, author: "Claude", content: "A thought", message_type: "thought"})
      create_message(%{room_id: room.id, author: "Gemini", content: "A critique", message_type: "critique"})

      {:ok, _view, html} = live(conn, "/rooms/type-room")
      assert html =~ "thought"
      assert html =~ "critique"
    end

    test "shows reply_to badge", %{conn: conn} do
      room = create_room(%{id: "reply-display"})
      m1 = create_message(%{room_id: room.id, author: "Claude", content: "Original"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Reply", reply_to: m1.id})

      {:ok, _view, html} = live(conn, "/rooms/reply-display")
      assert html =~ "re: ##{m1.id}"
    end

    test "shows summary blocks", %{conn: conn} do
      room = create_room(%{id: "summary-room"})
      create_message(%{room_id: room.id, author: "System", content: "Summary content", is_summary: true})

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
      create_message(%{room_id: room.id, author: "System", content: "Summary text", is_summary: true})

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

  describe "filter_rooms/2" do
    test "empty query returns all" do
      rooms = [%{id: "a", description: "Alpha"}, %{id: "b", description: "Beta"}]
      assert CouncilHubUiWeb.CouncilLive.filter_rooms(rooms, "") == rooms
    end

    test "filters by id and description" do
      rooms = [%{id: "auth-room", description: "Auth work"}, %{id: "db-room", description: "DB stuff"}]
      filtered = CouncilHubUiWeb.CouncilLive.filter_rooms(rooms, "auth")
      assert length(filtered) == 1
      assert hd(filtered).id == "auth-room"
    end

    test "case insensitive" do
      rooms = [%{id: "JWT-room", description: "JWT work"}]
      assert length(CouncilHubUiWeb.CouncilLive.filter_rooms(rooms, "jwt")) == 1
    end
  end
end
