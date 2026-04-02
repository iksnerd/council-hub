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

    test "returns all messages when last_id is empty string" do
      room = create_room(%{id: "all-since"})
      create_message(%{room_id: room.id, content: "First"})
      create_message(%{room_id: room.id, content: "Second"})

      msgs = Council.get_messages_since(room.id, "")
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
      msg = create_message(%{room_id: room.id, reply_to: "some-parent-uuid"})
      assert msg.reply_to == "some-parent-uuid"
    end

    test "reply_to defaults to empty string" do
      room = create_room(%{id: "reply-default"})
      msg = create_message(%{room_id: room.id})
      assert msg.reply_to == ""
    end
  end

  describe "list_messages_for_room/2 with type_filter" do
    test "filters messages by type" do
      room = create_room(%{id: "tf-room"})
      create_message(%{room_id: room.id, content: "A decision", message_type: "decision"})
      create_message(%{room_id: room.id, content: "An action", message_type: "action"})
      create_message(%{room_id: room.id, content: "A thought", message_type: "thought"})

      decisions = Council.list_messages_for_room(room.id, "decision")
      assert length(decisions) == 1
      assert hd(decisions).message_type == "decision"
    end

    test "always includes summaries regardless of filter" do
      room = create_room(%{id: "tf-sum-room"})
      create_message(%{room_id: room.id, content: "A decision", message_type: "decision"})
      create_message(%{room_id: room.id, content: "Summary", is_summary: true})

      decisions = Council.list_messages_for_room(room.id, "decision")
      contents = Enum.map(decisions, & &1.content)
      assert "A decision" in contents
      assert "Summary" in contents
    end

    test "all filter returns everything" do
      room = create_room(%{id: "tf-all-room"})
      create_message(%{room_id: room.id, content: "d", message_type: "decision"})
      create_message(%{room_id: room.id, content: "a", message_type: "action"})

      all = Council.list_messages_for_room(room.id, "all")
      assert length(all) == 2
    end
  end

  describe "get_messages_since/3 with type_filter" do
    test "filters new messages by type" do
      room = create_room(%{id: "gs-tf-room"})
      m1 = create_message(%{room_id: room.id, content: "Old decision", message_type: "decision"})
      _m2 = create_message(%{room_id: room.id, content: "New thought", message_type: "thought"})
      _m3 = create_message(%{room_id: room.id, content: "New decision", message_type: "decision"})

      msgs = Council.get_messages_since(room.id, m1.id, "decision")
      assert length(msgs) == 1
      assert hd(msgs).content == "New decision"
    end
  end

  describe "search_messages_in_room/2" do
    test "finds messages by content" do
      room = create_room(%{id: "search-room"})
      create_message(%{room_id: room.id, content: "Authentication fix", author: "Claude"})
      create_message(%{room_id: room.id, content: "Unrelated post", author: "Gemini"})

      results = Council.search_messages_in_room(room.id, "authentication")
      assert length(results) == 1
      assert hd(results).content == "Authentication fix"
    end

    test "search is case-insensitive" do
      room = create_room(%{id: "search-case"})
      create_message(%{room_id: room.id, content: "FaceID setup", author: "Claude"})

      assert length(Council.search_messages_in_room(room.id, "faceid")) == 1
      assert length(Council.search_messages_in_room(room.id, "FACEID")) == 1
    end

    test "returns empty list for blank query" do
      room = create_room(%{id: "search-blank"})
      create_message(%{room_id: room.id, content: "Some content"})

      assert Council.search_messages_in_room(room.id, "") == []
      assert Council.search_messages_in_room(room.id, nil) == []
    end

    test "only returns messages from the given room" do
      r1 = create_room(%{id: "search-r1"})
      r2 = create_room(%{id: "search-r2"})
      create_message(%{room_id: r1.id, content: "authentication"})
      create_message(%{room_id: r2.id, content: "authentication"})

      results = Council.search_messages_in_room(r1.id, "authentication")
      assert length(results) == 1
      assert hd(results).room_id == r1.id
    end
  end

  describe "all_room_participant_counts/0" do
    test "returns distinct author count per room" do
      r1 = create_room(%{id: "pc-r1"})
      create_message(%{room_id: r1.id, author: "Claude"})
      create_message(%{room_id: r1.id, author: "Gemini"})
      create_message(%{room_id: r1.id, author: "Claude"})

      counts = Council.all_room_participant_counts()
      assert counts[r1.id] == 2
    end

    test "returns empty map when no messages" do
      assert Council.all_room_participant_counts() == %{}
    end
  end

  describe "search_messages/1" do
    test "searches by content query" do
      room = create_room(%{id: "sm-query"})
      create_message(%{room_id: room.id, content: "erlang distribution", author: "Claude"})
      create_message(%{room_id: room.id, content: "unrelated topic", author: "Gemini"})

      results = Council.search_messages(%{"query" => "erlang"})
      assert length(results) == 1
      assert hd(results).content =~ "erlang"
    end

    test "filters by author" do
      room = create_room(%{id: "sm-author"})
      create_message(%{room_id: room.id, content: "msg 1", author: "Claude"})
      create_message(%{room_id: room.id, content: "msg 2", author: "Gemini"})

      results = Council.search_messages(%{"author" => "Claude", "room_id" => "sm-author"})
      assert length(results) == 1
      assert hd(results).author == "Claude"
    end

    test "filters by message_type" do
      room = create_room(%{id: "sm-type"})
      create_message(%{room_id: room.id, content: "a thought", message_type: "thought"})
      create_message(%{room_id: room.id, content: "a decision", message_type: "decision"})

      results = Council.search_messages(%{"message_type" => "decision", "room_id" => "sm-type"})
      assert length(results) == 1
      assert hd(results).message_type == "decision"
    end

    test "filters by project" do
      r1 = create_room(%{id: "sm-proj-a", project: "alpha"})
      r2 = create_room(%{id: "sm-proj-b", project: "beta"})
      create_message(%{room_id: r1.id, content: "in alpha"})
      create_message(%{room_id: r2.id, content: "in beta"})

      results = Council.search_messages(%{"project" => "alpha"})
      assert length(results) == 1
      assert hd(results).room_id == "sm-proj-a"
    end

    test "respects limit" do
      room = create_room(%{id: "sm-limit"})
      for i <- 1..5, do: create_message(%{room_id: room.id, content: "msg #{i}"})

      results = Council.search_messages(%{"room_id" => "sm-limit", "limit" => 2})
      assert length(results) == 2
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
  end

  describe "format_transcript/2" do
    test "formats room header and messages" do
      room = create_room(%{id: "fmt-room", description: "Format test", project: "proj", tech_stack: "Go", tags: "tag1", status: "active"})
      create_message(%{room_id: room.id, author: "Claude", content: "Hello", message_type: "thought"})
      create_message(%{room_id: room.id, author: "Gemini", content: "World", message_type: "message"})

      messages = Council.list_messages_for_room(room.id)
      transcript = Council.format_transcript(room, messages)

      assert transcript =~ "# COUNCIL ROOM: fmt-room"
      assert transcript =~ "**Project:** proj"
      assert transcript =~ "**Tech Stack:** Go"
      assert transcript =~ "**Tags:** tag1"
      assert transcript =~ "Claude (thought)"
      assert transcript =~ "Gemini:"
      assert transcript =~ "Hello"
      assert transcript =~ "World"
    end

    test "formats summary messages" do
      room = create_room(%{id: "sum-fmt", description: "Summary format"})
      create_message(%{room_id: room.id, author: "System", content: "Summary here", is_summary: true})

      messages = Council.list_messages_for_room(room.id)
      transcript = Council.format_transcript(room, messages)

      assert transcript =~ "SUMMARY"
      assert transcript =~ "Summary here"
    end

    test "formats reply_to" do
      room = create_room(%{id: "reply-fmt", description: "Reply format"})
      m1 = create_message(%{room_id: room.id, author: "Claude", content: "Original"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Reply", message_type: "review", reply_to: m1.id})

      messages = Council.list_messages_for_room(room.id)
      transcript = Council.format_transcript(room, messages)

      assert transcript =~ "re: ##{String.slice(m1.id, 0, 8)}"
    end

    test "includes system prompt" do
      room = create_room(%{id: "sys-fmt", description: "Sys prompt", system_prompt: "Be helpful"})

      transcript = Council.format_transcript(room, [])
      assert transcript =~ "*Instructions: Be helpful*"
    end

    test "includes related rooms" do
      room = create_room(%{id: "rel-fmt", description: "Related", related_rooms: "a,b"})

      transcript = Council.format_transcript(room, [])
      assert transcript =~ "**Related Rooms:** a,b"
    end
  end
end
