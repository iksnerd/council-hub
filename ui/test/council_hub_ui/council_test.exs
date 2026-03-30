defmodule CouncilHubUi.CouncilTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.Council
  import CouncilHubUi.CouncilFixtures

  describe "list_rooms/0" do
    test "returns empty list when no rooms" do
      assert Council.list_rooms() == []
    end

    test "returns all rooms ordered by updated_at desc" do
      _r1 = create_room(%{id: "room-a", updated_at: ~N[2026-03-28 10:00:00]})
      r2 = create_room(%{id: "room-b", updated_at: ~N[2026-03-29 10:00:00]})

      rooms = Council.list_rooms()
      assert length(rooms) == 2
      assert hd(rooms).id == r2.id
    end
  end

  describe "get_room/1" do
    test "returns room by id" do
      room = create_room(%{id: "my-room", description: "Test room"})
      found = Council.get_room("my-room")
      assert found.id == room.id
      assert found.description == "Test room"
    end

    test "returns nil for nonexistent room" do
      assert Council.get_room("nonexistent") == nil
    end
  end

  describe "list_messages_for_room/1" do
    test "returns messages for a room ordered by id" do
      room = create_room(%{id: "msg-room"})
      m1 = create_message(%{room_id: room.id, content: "First", author: "Claude"})
      m2 = create_message(%{room_id: room.id, content: "Second", author: "Gemini"})

      msgs = Council.list_messages_for_room(room.id)
      assert length(msgs) == 2
      assert hd(msgs).id == m1.id
      assert List.last(msgs).id == m2.id
    end

    test "returns empty list for room with no messages" do
      room = create_room(%{id: "empty-room"})
      assert Council.list_messages_for_room(room.id) == []
    end

    test "doesn't return messages from other rooms" do
      r1 = create_room(%{id: "room-1"})
      r2 = create_room(%{id: "room-2"})
      create_message(%{room_id: r1.id, content: "In room 1"})
      create_message(%{room_id: r2.id, content: "In room 2"})

      msgs = Council.list_messages_for_room(r1.id)
      assert length(msgs) == 1
      assert hd(msgs).content == "In room 1"
    end
  end

  describe "get_messages_since/2" do
    test "returns messages after given id" do
      room = create_room(%{id: "since-room"})
      m1 = create_message(%{room_id: room.id, content: "Old"})
      _m2 = create_message(%{room_id: room.id, content: "New"})

      msgs = Council.get_messages_since(room.id, m1.id)
      assert length(msgs) == 1
      assert hd(msgs).content == "New"
    end

    test "returns empty when no new messages" do
      room = create_room(%{id: "no-new"})
      m1 = create_message(%{room_id: room.id, content: "Only one"})

      assert Council.get_messages_since(room.id, m1.id) == []
    end

    test "returns all messages when last_id is 0" do
      room = create_room(%{id: "all-since"})
      create_message(%{room_id: room.id, content: "First"})
      create_message(%{room_id: room.id, content: "Second"})

      msgs = Council.get_messages_since(room.id, 0)
      assert length(msgs) == 2
    end
  end

  describe "all_room_message_counts/0" do
    test "returns counts per room" do
      r1 = create_room(%{id: "count-1"})
      r2 = create_room(%{id: "count-2"})
      create_message(%{room_id: r1.id})
      create_message(%{room_id: r1.id})
      create_message(%{room_id: r2.id})

      counts = Council.all_room_message_counts()
      assert counts[r1.id] == 2
      assert counts[r2.id] == 1
    end

    test "returns empty map when no messages" do
      assert Council.all_room_message_counts() == %{}
    end
  end

  describe "latest_room_update/0" do
    test "returns latest updated_at" do
      create_room(%{id: "old", updated_at: ~N[2026-03-28 10:00:00]})
      create_room(%{id: "new", updated_at: ~N[2026-03-29 15:00:00]})

      latest = Council.latest_room_update()
      assert latest == ~N[2026-03-29 15:00:00]
    end

    test "returns nil when no rooms" do
      assert Council.latest_room_update() == nil
    end
  end

  describe "room_participants/1" do
    test "returns unique authors" do
      room = create_room(%{id: "participants"})
      create_message(%{room_id: room.id, author: "Claude"})
      create_message(%{room_id: room.id, author: "Gemini"})
      create_message(%{room_id: room.id, author: "Claude"})

      participants = Council.room_participants(room.id)
      assert Enum.sort(participants) == ["Claude", "Gemini"]
    end

    test "excludes summary authors" do
      room = create_room(%{id: "no-summary"})
      create_message(%{room_id: room.id, author: "Claude"})
      create_message(%{room_id: room.id, author: "System", is_summary: true})

      participants = Council.room_participants(room.id)
      assert participants == ["Claude"]
    end

    test "returns empty list for empty room" do
      room = create_room(%{id: "empty-participants"})
      assert Council.room_participants(room.id) == []
    end
  end

  # -- Schema fields --

  describe "Room schema" do
    test "has related_rooms field" do
      room = create_room(%{id: "related-test", related_rooms: "room-a,room-b"})
      assert room.related_rooms == "room-a,room-b"
    end
  end

  describe "Message schema" do
    test "has reply_to field" do
      room = create_room(%{id: "reply-test"})
      msg = create_message(%{room_id: room.id, reply_to: 42})
      assert msg.reply_to == 42
    end

    test "reply_to defaults to 0" do
      room = create_room(%{id: "reply-default"})
      msg = create_message(%{room_id: room.id})
      assert msg.reply_to == 0
    end
  end
end
