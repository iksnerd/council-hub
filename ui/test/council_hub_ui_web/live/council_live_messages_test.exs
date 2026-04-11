defmodule CouncilHubUiWeb.CouncilLiveMessagesTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest
  import CouncilHubUi.CouncilFixtures

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

    test "reply badge has ScrollToMessage hook with full reply_to id", %{conn: conn} do
      room = create_room(%{id: "reply-hook"})
      m1 = create_message(%{room_id: room.id, author: "Claude", content: "Original"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Reply", reply_to: m1.id})

      {:ok, _view, html} = live(conn, "/rooms/reply-hook")
      assert html =~ ~s(phx-hook="ScrollToMessage")
      assert html =~ ~s(data-reply-to="#{m1.id}")
    end

    test "each message has a msg-id anchor for scroll targeting", %{conn: conn} do
      room = create_room(%{id: "scroll-anchors"})
      m1 = create_message(%{room_id: room.id, author: "Claude", content: "Msg one"})

      {:ok, _view, html} = live(conn, "/rooms/scroll-anchors")
      assert html =~ ~s(id="msg-#{m1.id}")
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

      # Click collapse toggle — summary id gets added to collapsed_summaries MapSet
      view |> element("button[phx-click='toggle_summary']") |> render_click()
      state = :sys.get_state(view.pid).socket.assigns
      assert MapSet.size(state.collapsed_summaries) == 1

      # Click again to expand — removed from collapsed set
      view |> element("button[phx-click='toggle_summary']") |> render_click()
      state = :sys.get_state(view.pid).socket.assigns
      assert MapSet.size(state.collapsed_summaries) == 0
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
end
