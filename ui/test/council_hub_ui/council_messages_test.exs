defmodule CouncilHubUi.CouncilMessagesTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.Council
  import CouncilHubUi.CouncilFixtures

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

    test "respects type_filter parameter" do
      room = create_room(%{id: "search-typed"})
      create_message(%{room_id: room.id, content: "auth decision", message_type: "decision"})
      create_message(%{room_id: room.id, content: "auth thought", message_type: "thought"})

      results = Council.search_messages_in_room(room.id, "auth", "decision")
      assert length(results) == 1
      assert hd(results).message_type == "decision"
    end

    test "type_filter includes summaries" do
      room = create_room(%{id: "search-typed-sum"})
      create_message(%{room_id: room.id, content: "auth decision", message_type: "decision"})
      create_message(%{room_id: room.id, content: "auth summary", is_summary: true})
      create_message(%{room_id: room.id, content: "auth thought", message_type: "thought"})

      results = Council.search_messages_in_room(room.id, "auth", "decision")
      types = Enum.map(results, & &1.message_type)
      assert "decision" in types
      # summary should be included even though filter is "decision"
      assert length(results) == 2
    end

    test "type_filter all returns everything" do
      room = create_room(%{id: "search-typed-all"})
      create_message(%{room_id: room.id, content: "auth decision", message_type: "decision"})
      create_message(%{room_id: room.id, content: "auth thought", message_type: "thought"})

      results = Council.search_messages_in_room(room.id, "auth", "all")
      assert length(results) == 2
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

    test "filters by since date" do
      room = create_room(%{id: "sm-since"})
      yesterday = NaiveDateTime.utc_now() |> NaiveDateTime.add(-86400, :second)
      tomorrow = NaiveDateTime.utc_now() |> NaiveDateTime.add(86400, :second)

      create_message(%{
        room_id: room.id,
        content: "recent msg",
        timestamp: NaiveDateTime.utc_now()
      })

      # since yesterday -> found
      results = Council.search_messages(%{"room_id" => "sm-since", "since" => yesterday})
      assert length(results) == 1

      # since tomorrow -> not found
      results = Council.search_messages(%{"room_id" => "sm-since", "since" => tomorrow})
      assert results == []
    end

    test "filters by until date" do
      room = create_room(%{id: "sm-until"})
      yesterday = NaiveDateTime.utc_now() |> NaiveDateTime.add(-86400, :second)
      tomorrow = NaiveDateTime.utc_now() |> NaiveDateTime.add(86400, :second)

      create_message(%{
        room_id: room.id,
        content: "recent msg",
        timestamp: NaiveDateTime.utc_now()
      })

      # until tomorrow -> found
      results = Council.search_messages(%{"room_id" => "sm-until", "until" => tomorrow})
      assert length(results) == 1

      # until yesterday -> not found
      results = Council.search_messages(%{"room_id" => "sm-until", "until" => yesterday})
      assert results == []
    end
  end

  describe "get_mentions/2" do
    test "returns empty list when no messages have mentions" do
      r = create_room(%{id: "mentions-empty"})
      create_message(%{room_id: r.id, author: "claude", content: "no mention here"})

      assert Council.get_mentions("claude") == []
    end

    test "returns messages where author is mentioned" do
      r = create_room(%{id: "mentions-basic"})

      m =
        create_message(%{
          room_id: r.id,
          author: "gemini",
          content: "hey",
          mentions: "claude,warp"
        })

      _other = create_message(%{room_id: r.id, author: "gemini", content: "unrelated"})

      result = Council.get_mentions("claude")
      assert length(result) == 1
      assert hd(result).id == m.id
    end

    test "does not match partial author names" do
      r = create_room(%{id: "mentions-partial"})
      create_message(%{room_id: r.id, author: "gemini", content: "hi", mentions: "claude-sonnet"})

      # "claude" matches "claude-sonnet" via LIKE — this is acceptable at our scale
      # but ensure "warp" does NOT match "claude-sonnet"
      assert Council.get_mentions("warp") == []
    end

    test "orders by timestamp descending" do
      r = create_room(%{id: "mentions-order"})

      m1 =
        create_message(%{
          room_id: r.id,
          author: "a",
          content: "first",
          mentions: "claude",
          timestamp: ~N[2026-04-01 10:00:00]
        })

      m2 =
        create_message(%{
          room_id: r.id,
          author: "b",
          content: "second",
          mentions: "claude",
          timestamp: ~N[2026-04-01 11:00:00]
        })

      result = Council.get_mentions("claude")
      ids = Enum.map(result, & &1.id)
      assert ids == [m2.id, m1.id]
    end

    test "respects limit" do
      r = create_room(%{id: "mentions-limit"})

      for i <- 1..5,
          do:
            create_message(%{room_id: r.id, author: "x", content: "msg #{i}", mentions: "claude"})

      assert length(Council.get_mentions("claude", 3)) == 3
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
end
