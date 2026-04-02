defmodule CouncilHubUi.ClusterTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.Cluster
  import CouncilHubUi.CouncilFixtures

  describe "search_messages/1" do
    test "returns local results on single node" do
      room = create_room(%{id: "cluster-search"})
      create_message(%{room_id: room.id, author: "Claude", content: "distributed erlang"})
      create_message(%{room_id: room.id, author: "Gemini", content: "unrelated"})

      result = Cluster.search_messages(%{"query" => "distributed", "limit" => 20})

      assert length(result.results) == 1
      assert hd(result.results).content =~ "distributed"
      assert hd(result.results).source_node == Atom.to_string(Node.self())
      assert result.warnings == []
    end

    test "respects limit" do
      room = create_room(%{id: "cluster-limit"})
      for i <- 1..5, do: create_message(%{room_id: room.id, content: "msg #{i}"})

      result = Cluster.search_messages(%{"room_id" => "cluster-limit", "limit" => 2})

      assert length(result.results) == 2
    end

    test "returns empty results when no match" do
      result = Cluster.search_messages(%{"query" => "nonexistent-query-xyz"})

      assert result.results == []
      assert result.warnings == []
    end

    test "filters by author" do
      room = create_room(%{id: "cluster-author"})
      create_message(%{room_id: room.id, author: "Claude", content: "from claude"})
      create_message(%{room_id: room.id, author: "Gemini", content: "from gemini"})

      result = Cluster.search_messages(%{"author" => "Claude", "room_id" => "cluster-author"})
      assert length(result.results) == 1
      assert hd(result.results).author == "Claude"
    end

    test "combines multiple filters" do
      room = create_room(%{id: "cluster-multi", project: "proj-multi"})
      create_message(%{room_id: room.id, author: "Claude", content: "decision about auth", message_type: "decision"})
      create_message(%{room_id: room.id, author: "Claude", content: "thought about auth", message_type: "thought"})
      create_message(%{room_id: room.id, author: "Gemini", content: "decision about db", message_type: "decision"})

      result = Cluster.search_messages(%{
        "query" => "auth",
        "author" => "Claude",
        "message_type" => "decision",
        "room_id" => "cluster-multi"
      })

      assert length(result.results) == 1
      assert hd(result.results).content =~ "decision about auth"
    end

    test "results are sorted by timestamp descending" do
      room = create_room(%{id: "cluster-sort"})
      create_message(%{room_id: room.id, content: "first"})
      create_message(%{room_id: room.id, content: "second"})
      create_message(%{room_id: room.id, content: "third"})

      result = Cluster.search_messages(%{"room_id" => "cluster-sort", "limit" => 50})
      contents = Enum.map(result.results, & &1.content)
      # Descending = newest first
      assert hd(contents) == "third"
      assert List.last(contents) == "first"
    end
  end

  describe "list_rooms/1" do
    test "returns local rooms on single node" do
      create_room(%{id: "cluster-room-a", project: "proj-x"})
      create_room(%{id: "cluster-room-b", project: "proj-y"})

      result = Cluster.list_rooms(%{"project" => "proj-x"})

      assert length(result.results) == 1
      assert hd(result.results).id == "cluster-room-a"
      assert hd(result.results).source_node == Atom.to_string(Node.self())
      assert result.warnings == []
    end

    test "returns all rooms with empty filters" do
      create_room(%{id: "cluster-all-a"})
      create_room(%{id: "cluster-all-b"})

      result = Cluster.list_rooms(%{})

      ids = Enum.map(result.results, & &1.id)
      assert "cluster-all-a" in ids
      assert "cluster-all-b" in ids
    end

    test "filters by status" do
      create_room(%{id: "cluster-active", status: "active"})
      create_room(%{id: "cluster-resolved", status: "resolved"})

      result = Cluster.list_rooms(%{"status" => "resolved"})
      assert length(result.results) == 1
      assert hd(result.results).id == "cluster-resolved"
    end

    test "filters by tag" do
      create_room(%{id: "cluster-tagged", tags: "elixir,erlang"})
      create_room(%{id: "cluster-untagged", tags: "go"})

      result = Cluster.list_rooms(%{"tag" => "erlang"})
      assert length(result.results) == 1
      assert hd(result.results).id == "cluster-tagged"
    end

    test "searches by keyword" do
      create_room(%{id: "cluster-kw-match", description: "distributed erlang setup"})
      create_room(%{id: "cluster-kw-other", description: "unrelated stuff"})

      result = Cluster.list_rooms(%{"search" => "erlang"})
      assert length(result.results) == 1
      assert hd(result.results).id == "cluster-kw-match"
    end

    test "returns empty when no rooms match" do
      result = Cluster.list_rooms(%{"project" => "nonexistent-project-zzz"})
      assert result.results == []
    end
  end

  describe "room_stats/1" do
    test "returns stats for existing room" do
      room = create_room(%{id: "cluster-stats"})
      create_message(%{room_id: room.id, author: "Claude", message_type: "thought"})
      create_message(%{room_id: room.id, author: "Gemini", message_type: "decision"})

      result = Cluster.room_stats("cluster-stats")

      assert result.results != nil
      assert result.results.room_id == "cluster-stats"
      assert result.results.message_count == 2
      assert result.results.source_node == Atom.to_string(Node.self())
      assert result.warnings == []
    end

    test "returns nil results for nonexistent room" do
      result = Cluster.room_stats("nonexistent-room")

      assert result.results == nil
    end

    test "nonexistent room generates a warning" do
      result = Cluster.room_stats("missing-room-xyz")

      assert result.results == nil
      assert length(result.warnings) == 1
      assert hd(result.warnings) =~ "not found"
    end

    test "stats include correct participant breakdown" do
      room = create_room(%{id: "cluster-stats-parts"})
      create_message(%{room_id: room.id, author: "Claude"})
      create_message(%{room_id: room.id, author: "Claude"})
      create_message(%{room_id: room.id, author: "Gemini"})

      result = Cluster.room_stats("cluster-stats-parts")
      assert result.results.participants == %{"Claude" => 2, "Gemini" => 1}
    end
  end

  describe "read_transcript/1" do
    test "returns transcript data for existing room" do
      room = create_room(%{id: "cluster-transcript-room"})
      create_message(%{room_id: room.id, content: "First", author: "Claude"})
      create_message(%{room_id: room.id, content: "Second", author: "Gemini", pinned: true})

      result = Cluster.read_transcript(room.id)
      
      assert result.results != nil
      assert result.results.room.id == room.id
      assert length(result.results.messages) == 2
      assert result.results.pinned != nil
      assert result.results.source_node == Atom.to_string(Node.self())
    end

    test "returns nil results for nonexistent room" do
      result = Cluster.read_transcript("nonexistent-room")
      
      assert result.results == nil
      assert length(result.warnings) == 1
      assert hd(result.warnings) =~ "not found"
    end
  end
end
