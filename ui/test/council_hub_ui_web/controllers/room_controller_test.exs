defmodule CouncilHubUiWeb.RoomControllerTest do
  use CouncilHubUiWeb.ConnCase

  import CouncilHubUi.CouncilFixtures

  describe "export" do
    test "returns markdown transcript as download", %{conn: conn} do
      room = create_room(%{id: "export-room", description: "Export test", project: "proj"})
      create_message(%{room_id: room.id, author: "Claude", content: "Hello world", message_type: "thought"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Reply here", message_type: "review"})

      conn = get(conn, "/rooms/export-room/export")

      assert conn.status == 200
      assert get_resp_header(conn, "content-type") |> hd() =~ "text/markdown"
      assert get_resp_header(conn, "content-disposition") |> hd() =~ "export-room.md"

      body = conn.resp_body
      assert body =~ "# COUNCIL ROOM: export-room"
      assert body =~ "**Project:** proj"
      assert body =~ "**Topic:** Export test"
      assert body =~ "Claude (thought)"
      assert body =~ "Gemini (review)"
      assert body =~ "Hello world"
      assert body =~ "Reply here"
    end

    test "includes related rooms in transcript", %{conn: conn} do
      create_room(%{id: "linked-export", description: "Linked", related_rooms: "room-a,room-b"})

      conn = get(conn, "/rooms/linked-export/export")
      assert conn.resp_body =~ "**Related Rooms:** room-a,room-b"
    end

    test "includes reply_to in transcript", %{conn: conn} do
      room = create_room(%{id: "reply-export", description: "Reply test"})
      m1 = create_message(%{room_id: room.id, author: "Claude", content: "Original"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Reply", message_type: "review", reply_to: m1.id})

      conn = get(conn, "/rooms/reply-export/export")
      assert conn.resp_body =~ "re: ##{String.slice(m1.id, 0, 8)}"
    end

    test "includes system prompt in transcript", %{conn: conn} do
      create_room(%{id: "prompt-export", description: "Prompt test", system_prompt: "Be concise"})

      conn = get(conn, "/rooms/prompt-export/export")
      assert conn.resp_body =~ "*Instructions: Be concise*"
    end

    test "returns 404 for nonexistent room", %{conn: conn} do
      conn = get(conn, "/rooms/nonexistent/export")
      assert conn.status == 404
      assert conn.resp_body =~ "not found"
    end

    test "exports empty room with no messages", %{conn: conn} do
      create_room(%{id: "empty-export", description: "Empty room"})

      conn = get(conn, "/rooms/empty-export/export")
      assert conn.status == 200
      assert conn.resp_body =~ "# COUNCIL ROOM: empty-export"
    end
  end
end
