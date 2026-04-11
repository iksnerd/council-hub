defmodule CouncilHubUi.CouncilRoomsTest do
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

  describe "get_room_with_messages/1" do
    test "returns room, messages, and pinned message" do
      room = create_room(%{id: "room-with-msgs"})
      m1 = create_message(%{room_id: room.id, content: "First", author: "Claude"})
      m2 = create_message(%{room_id: room.id, content: "Second", author: "Gemini", pinned: true})

      assert {:ok, result} = Council.get_room_with_messages(room.id)
      assert result.room.id == room.id
      assert length(result.messages) == 2
      assert Enum.at(result.messages, 0).id == m1.id
      assert Enum.at(result.messages, 1).id == m2.id
      assert result.pinned != nil
      assert result.pinned.id == m2.id
    end

    test "returns error for nonexistent room" do
      assert {:error, "room 'nonexistent' not found"} =
               Council.get_room_with_messages("nonexistent")
    end
  end

  describe "list_rooms_filtered/1" do
    test "filters by project" do
      create_room(%{id: "lrf-a", project: "proj-x"})
      create_room(%{id: "lrf-b", project: "proj-y"})

      results = Council.list_rooms_filtered(%{"project" => "proj-x"})
      assert length(results) == 1
      assert hd(results).id == "lrf-a"
    end

    test "filters by status" do
      create_room(%{id: "lrf-active", status: "active"})
      create_room(%{id: "lrf-resolved", status: "resolved"})

      results = Council.list_rooms_filtered(%{"status" => "resolved"})
      assert length(results) == 1
      assert hd(results).id == "lrf-resolved"
    end

    test "filters by tag" do
      create_room(%{id: "lrf-tagged", tags: "elixir,erlang"})
      create_room(%{id: "lrf-other", tags: "go,rust"})

      results = Council.list_rooms_filtered(%{"tag" => "erlang"})
      assert length(results) == 1
      assert hd(results).id == "lrf-tagged"
    end

    test "searches across id, description, and tags" do
      create_room(%{id: "auth-migration", description: "Auth work", tags: "security"})
      create_room(%{id: "unrelated", description: "Other", tags: ""})

      results = Council.list_rooms_filtered(%{"search" => "auth"})
      assert length(results) == 1
      assert hd(results).id == "auth-migration"
    end

    test "multi-word search matches all words (AND logic)" do
      create_room(%{id: "council-hub-test", description: "Main hub room", tags: "mcp"})
      create_room(%{id: "unrelated-room", description: "Other", tags: ""})

      # Both words present in the id
      results = Council.list_rooms_filtered(%{"search" => "council hub"})
      assert length(results) == 1
      assert hd(results).id == "council-hub-test"

      # One word doesn't match anything relevant
      results = Council.list_rooms_filtered(%{"search" => "council nonexistent"})
      assert results == []
    end

    test "returns all rooms with empty filters" do
      create_room(%{id: "lrf-all-a"})
      create_room(%{id: "lrf-all-b"})

      results = Council.list_rooms_filtered(%{})
      ids = Enum.map(results, & &1.id)
      assert "lrf-all-a" in ids
      assert "lrf-all-b" in ids
    end
  end

  describe "room_stats/1" do
    test "returns stats for existing room" do
      room = create_room(%{id: "rs-room"})
      create_message(%{room_id: room.id, author: "Claude", message_type: "thought"})
      create_message(%{room_id: room.id, author: "Gemini", message_type: "decision"})
      create_message(%{room_id: room.id, author: "Claude", message_type: "thought"})

      assert {:ok, stats} = Council.room_stats("rs-room")
      assert stats.room_id == "rs-room"
      assert stats.message_count == 3
      assert stats.participants == %{"Claude" => 2, "Gemini" => 1}
      assert stats.type_counts == %{"thought" => 2, "decision" => 1}
      assert stats.first_message != nil
      assert stats.last_message != nil
      assert stats.latest_message_id != nil
    end

    test "returns error for nonexistent room" do
      assert {:error, msg} = Council.room_stats("nonexistent")
      assert msg =~ "not found"
    end

    test "returns zero counts for room with no messages" do
      create_room(%{id: "rs-empty", status: "active"})
      assert {:ok, stats} = Council.room_stats("rs-empty")
      assert stats.message_count == 0
      assert stats.participants == %{}
      assert stats.type_counts == %{}
      assert stats.status == "active"
    end
  end

  describe "Room schema" do
    test "has related_rooms field" do
      room = create_room(%{id: "related-test", related_rooms: "room-a,room-b"})
      assert room.related_rooms == "room-a,room-b"
    end
  end
end
