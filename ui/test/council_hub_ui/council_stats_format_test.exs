defmodule CouncilHubUi.CouncilStatsFormatTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.Council
  import CouncilHubUi.CouncilFixtures

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

  describe "all_room_key_type_counts/0" do
    test "returns decision and action counts per room" do
      room = create_room(%{id: "ktc-room"})
      create_message(%{room_id: room.id, message_type: "decision", author: "Claude"})
      create_message(%{room_id: room.id, message_type: "decision", author: "Gemini"})
      create_message(%{room_id: room.id, message_type: "action", author: "Claude"})
      create_message(%{room_id: room.id, message_type: "thought", author: "Claude"})

      counts = Council.all_room_key_type_counts()
      assert counts[room.id]["decision"] == 2
      assert counts[room.id]["action"] == 1
      refute Map.has_key?(counts[room.id] || %{}, "thought")
    end

    test "returns empty map when no decisions or actions" do
      room = create_room(%{id: "ktc-empty"})
      create_message(%{room_id: room.id, message_type: "thought", author: "Claude"})

      counts = Council.all_room_key_type_counts()
      assert counts[room.id] == nil
    end
  end

  describe "all_room_latest_message_ids/0" do
    test "returns empty map when no messages" do
      assert Council.all_room_latest_message_ids() == %{}
    end

    test "returns latest message id per room" do
      r1 = create_room(%{id: "latest-r1"})
      r2 = create_room(%{id: "latest-r2"})
      _m1 = create_message(%{room_id: r1.id, content: "first"})
      m2 = create_message(%{room_id: r1.id, content: "second"})
      m3 = create_message(%{room_id: r2.id, content: "only"})

      result = Council.all_room_latest_message_ids()
      assert result[r1.id] == m2.id
      assert result[r2.id] == m3.id
    end

    test "only includes rooms that have messages" do
      r1 = create_room(%{id: "has-msgs"})
      r2 = create_room(%{id: "no-msgs"})
      create_message(%{room_id: r1.id, content: "a message"})

      result = Council.all_room_latest_message_ids()
      assert Map.has_key?(result, r1.id)
      refute Map.has_key?(result, r2.id)
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

  describe "format_transcript/2" do
    test "formats room header and messages" do
      room =
        create_room(%{
          id: "fmt-room",
          description: "Format test",
          project: "proj",
          tech_stack: "Go",
          tags: "tag1",
          status: "active"
        })

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "Hello",
        message_type: "thought"
      })

      create_message(%{
        room_id: room.id,
        author: "Gemini",
        content: "World",
        message_type: "message"
      })

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

      create_message(%{
        room_id: room.id,
        author: "System",
        content: "Summary here",
        is_summary: true
      })

      messages = Council.list_messages_for_room(room.id)
      transcript = Council.format_transcript(room, messages)

      assert transcript =~ "SUMMARY"
      assert transcript =~ "Summary here"
    end

    test "formats reply_to" do
      room = create_room(%{id: "reply-fmt", description: "Reply format"})
      m1 = create_message(%{room_id: room.id, author: "Claude", content: "Original"})

      create_message(%{
        room_id: room.id,
        author: "Gemini",
        content: "Reply",
        message_type: "review",
        reply_to: m1.id
      })

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
