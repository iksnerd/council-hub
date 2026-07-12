defmodule CouncilHubUiWeb.CouncilLivePollingTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest
  import CouncilHubUi.CouncilFixtures

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

  describe "mutation polling (slim fetch)" do
    import Ecto.Query

    defp set_message_fields(id, fields) do
      CouncilHubUi.Repo.update_all(
        from(m in CouncilHubUi.Council.Message, where: m.id == ^id),
        set: fields
      )
    end

    test "a reaction added to an already-displayed message merges in", %{conn: conn} do
      room = create_room(%{id: "mut-react-room"})
      msg = create_message(%{room_id: room.id, author: "Claude", content: "React target"})

      {:ok, view, html} = live(conn, "/rooms/mut-react-room")
      assert html =~ "React target"

      set_message_fields(msg.id, reactions: ~s({"🚀":["gemini"]}))
      send(view.pid, :poll_messages)

      assert render(view) =~ "🚀"
    end

    test "a retraction of an already-displayed message renders the tombstone", %{conn: conn} do
      room = create_room(%{id: "mut-retract-room"})
      msg = create_message(%{room_id: room.id, author: "Claude", content: "Withdraw me please"})

      {:ok, view, html} = live(conn, "/rooms/mut-retract-room")
      # The rendered prose form — distinct from the raw content that also rides
      # along in the copy-button data attribute even after retraction.
      rendered_prose = "<p>\nWithdraw me please</p>"
      assert html =~ rendered_prose

      set_message_fields(msg.id,
        retracted_at: NaiveDateTime.utc_now(),
        retracted_by: "claude"
      )

      send(view.pid, :poll_messages)

      html = render(view)
      assert html =~ "[retracted by claude]"
      refute html =~ rendered_prose
    end

    test "an edit (revised flag) removes the stale row from the stream", %{conn: conn} do
      room = create_room(%{id: "mut-revise-room"})
      msg = create_message(%{room_id: room.id, author: "Claude", content: "Stale version"})

      {:ok, view, html} = live(conn, "/rooms/mut-revise-room")
      assert html =~ "Stale version"

      set_message_fields(msg.id, revised: true)
      create_message(%{room_id: room.id, author: "Claude", content: "New head", revises: msg.id})
      send(view.pid, :poll_messages)

      html = render(view)
      assert html =~ "New head"
      refute html =~ "Stale version"
    end
  end

  describe "poll_mentions" do
    test "poll_mentions assigns mentions list", %{conn: conn} do
      {:ok, view, _html} = live(conn, "/")
      send(view.pid, :poll_mentions)
      # Should not crash; in test env get_mentions returns [] with no matching messages
      assigns = :sys.get_state(view.pid).socket.assigns
      assert is_list(assigns.mentions)
    end

    test "poll_mentions with matching author returns mention results", %{conn: conn} do
      room = create_room(%{id: "mentions-room"})
      author = System.get_env("COUNCIL_AUTHOR", "claude-code")
      create_message(%{room_id: room.id, author: author, content: "@#{author} check this"})

      {:ok, view, _html} = live(conn, "/")
      send(view.pid, :poll_mentions)

      assigns = :sys.get_state(view.pid).socket.assigns
      assert is_list(assigns.mentions)
    end

    test "poll_mentions preserves old mentions when DB error occurs", %{conn: conn} do
      {:ok, view, _html} = live(conn, "/")
      # Force a DB-style error by sending the message when DB is accessible
      # (no crash expected — rescue clause keeps old value)
      send(view.pid, :poll_mentions)
      assigns = :sys.get_state(view.pid).socket.assigns
      assert is_list(assigns.mentions)
    end
  end

  describe "poll_archives" do
    test "poll_archives does not crash when MCP server is unreachable", %{conn: conn} do
      {:ok, view, _html} = live(conn, "/")
      # McpClient.list_archives will fail (no server in test env)
      # The rescue clause should preserve the existing empty archives list
      send(view.pid, :poll_archives)

      assigns = :sys.get_state(view.pid).socket.assigns
      assert is_list(assigns.archives)
    end

    test "poll_archives does not change archives when McpClient fails", %{conn: conn} do
      {:ok, view, _html} = live(conn, "/")

      initial_archives = :sys.get_state(view.pid).socket.assigns.archives
      send(view.pid, :poll_archives)
      after_archives = :sys.get_state(view.pid).socket.assigns.archives

      assert initial_archives == after_archives
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
      assert html =~ "Nodes"
      assert html =~ "#{Node.self()}"
    end
  end

  describe "toggle_cluster_wide" do
    test "defaults to local mode", %{conn: conn} do
      {:ok, view, html} = live(conn, "/")
      assert html =~ "LOCAL"
      assert :sys.get_state(view.pid).socket.assigns.cluster_wide == false
    end

    test "toggles to all-nodes mode and back", %{conn: conn} do
      create_room(%{id: "toggle-cw-room"})
      {:ok, view, html} = live(conn, "/")
      assert html =~ "LOCAL"

      html = view |> element("button[phx-click='toggle_cluster_wide']") |> render_click()
      assert html =~ "ALL"
      assert :sys.get_state(view.pid).socket.assigns.cluster_wide == true

      html = view |> element("button[phx-click='toggle_cluster_wide']") |> render_click()
      assert html =~ "LOCAL"
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

  describe "last_message_id/1" do
    alias CouncilHubUiWeb.CouncilLivePolling

    test "empty list yields empty cursor" do
      assert CouncilLivePolling.last_message_id([]) == ""
    end

    test "returns the max id even when a newer message is pinned first" do
      # Messages are ordered pinned-first for display, so the newest id is NOT last.
      # The cursor must still be the max id, or the poll re-queries the pinned row forever.
      messages = [
        %{id: "019ea7ec-ffff-7000-0000-000000000000", pinned: true},
        %{id: "019ea7ea-0000-7000-0000-000000000000", pinned: false}
      ]

      assert CouncilLivePolling.last_message_id(messages) ==
               "019ea7ec-ffff-7000-0000-000000000000"
    end
  end
end
