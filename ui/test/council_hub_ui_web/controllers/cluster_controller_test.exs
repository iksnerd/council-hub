defmodule CouncilHubUiWeb.ClusterControllerTest do
  use CouncilHubUiWeb.ConnCase

  import CouncilHubUi.CouncilFixtures

  describe "POST /api/internal/cluster/search_messages" do
    test "returns search results as JSON", %{conn: conn} do
      room = create_room(%{id: "api-search"})
      create_message(%{room_id: room.id, author: "Claude", content: "finding this message"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{"query" => "finding"})

      assert %{"results" => results, "warnings" => []} = json_response(conn, 200)
      assert length(results) == 1
      assert hd(results)["content"] =~ "finding"
      assert hd(results)["source_node"] != nil
    end

    test "returns empty results for no match", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{"query" => "zzz-no-match"})

      assert %{"results" => [], "warnings" => []} = json_response(conn, 200)
    end

    test "results include all expected fields", %{conn: conn} do
      room = create_room(%{id: "api-fields"})

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "test fields",
        message_type: "decision"
      })

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{"query" => "test fields"})

      %{"results" => [msg]} = json_response(conn, 200)
      assert msg["id"] != nil
      assert msg["room_id"] == "api-fields"
      assert msg["author"] == "Claude"
      assert msg["content"] == "test fields"
      assert msg["message_type"] == "decision"
      assert msg["timestamp"] != nil
      assert msg["source_node"] != nil
      assert is_boolean(msg["is_summary"])
      assert is_boolean(msg["pinned"])
    end

    test "filters by author and message_type", %{conn: conn} do
      room = create_room(%{id: "api-filter"})

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "claude thought",
        message_type: "thought"
      })

      create_message(%{
        room_id: room.id,
        author: "Gemini",
        content: "gemini thought",
        message_type: "thought"
      })

      create_message(%{
        room_id: room.id,
        author: "Claude",
        content: "claude msg",
        message_type: "message"
      })

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{
          "author" => "Claude",
          "message_type" => "thought",
          "room_id" => "api-filter"
        })

      %{"results" => results} = json_response(conn, 200)
      assert length(results) == 1
      assert hd(results)["content"] == "claude thought"
    end

    test "respects limit param", %{conn: conn} do
      room = create_room(%{id: "api-limit"})
      for i <- 1..5, do: create_message(%{room_id: room.id, content: "msg #{i}"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{
          "room_id" => "api-limit",
          "limit" => "2"
        })

      %{"results" => results} = json_response(conn, 200)
      assert length(results) == 2
    end
  end

  describe "POST /api/internal/cluster/search_messages — extended filters" do
    test "filters by room_ids (comma-separated)", %{conn: conn} do
      room_a = create_room(%{id: "api-filter-a"})
      room_b = create_room(%{id: "api-filter-b"})
      create_message(%{room_id: room_a.id, author: "Claude", content: "in room a"})
      create_message(%{room_id: room_b.id, author: "Claude", content: "in room b"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{
          "room_ids" => "api-filter-a"
        })

      %{"results" => results} = json_response(conn, 200)
      assert Enum.all?(results, &(&1["room_id"] == "api-filter-a"))
    end

    test "filters by project", %{conn: conn} do
      room = create_room(%{id: "api-proj-filter", project: "filter-proj"})
      create_message(%{room_id: room.id, content: "project message"})
      other_room = create_room(%{id: "api-other-proj", project: "other-proj"})
      create_message(%{room_id: other_room.id, content: "other project message"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{"project" => "filter-proj"})

      %{"results" => results} = json_response(conn, 200)
      assert Enum.all?(results, &(&1["room_id"] == "api-proj-filter"))
    end

    test "parse_limit clamps 0 to default", %{conn: conn} do
      room = create_room(%{id: "api-limit-zero"})
      for _ <- 1..5, do: create_message(%{room_id: room.id})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{
          "room_id" => "api-limit-zero",
          "limit" => "0"
        })

      # 0 is not > 0, so default 20 applies — all 5 messages returned
      %{"results" => results} = json_response(conn, 200)
      assert length(results) == 5
    end

    test "parse_limit clamps non-numeric to default", %{conn: conn} do
      room = create_room(%{id: "api-limit-abc"})
      for _ <- 1..3, do: create_message(%{room_id: room.id})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{
          "room_id" => "api-limit-abc",
          "limit" => "abc"
        })

      %{"results" => results} = json_response(conn, 200)
      assert length(results) == 3
    end

    test "parse_limit clamps >100 to 100", %{conn: conn} do
      room = create_room(%{id: "api-limit-over"})
      for _ <- 1..5, do: create_message(%{room_id: room.id})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/search_messages", %{
          "room_id" => "api-limit-over",
          "limit" => "999"
        })

      # 999 > 100 so default 20 applies — all 5 returned
      %{"results" => results} = json_response(conn, 200)
      assert length(results) == 5
    end
  end

  describe "POST /api/internal/cluster/list_rooms" do
    test "returns rooms as JSON", %{conn: conn} do
      create_room(%{id: "api-room-a", project: "api-proj"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/list_rooms", %{"project" => "api-proj"})

      assert %{"results" => results, "warnings" => []} = json_response(conn, 200)
      assert length(results) == 1
      assert hd(results)["id"] == "api-room-a"
    end

    test "room results include all expected fields", %{conn: conn} do
      create_room(%{
        id: "api-room-fields",
        project: "p",
        tech_stack: "Go",
        tags: "tag1",
        description: "desc"
      })

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/list_rooms", %{"search" => "api-room-fields"})

      %{"results" => [room]} = json_response(conn, 200)
      assert room["id"] == "api-room-fields"
      assert room["project"] == "p"
      assert room["tech_stack"] == "Go"
      assert room["tags"] == "tag1"
      assert room["description"] == "desc"
      assert room["status"] == "active"
      assert room["created_at"] != nil
      assert room["updated_at"] != nil
      assert room["source_node"] != nil
    end

    test "returns empty for no match", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/list_rooms", %{"project" => "nonexistent-zzz"})

      assert %{"results" => [], "warnings" => []} = json_response(conn, 200)
    end

    test "filters by tag", %{conn: conn} do
      create_room(%{id: "api-room-tagged", tags: "featured"})
      create_room(%{id: "api-room-untagged", tags: ""})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/list_rooms", %{"tag" => "featured"})

      %{"results" => results} = json_response(conn, 200)
      ids = Enum.map(results, & &1["id"])
      assert "api-room-tagged" in ids
      refute "api-room-untagged" in ids
    end

    test "filters by status", %{conn: conn} do
      create_room(%{id: "api-room-resolved", status: "resolved"})
      create_room(%{id: "api-room-active-status", status: "active"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/list_rooms", %{"status" => "resolved"})

      %{"results" => results} = json_response(conn, 200)
      ids = Enum.map(results, & &1["id"])
      assert "api-room-resolved" in ids
      refute "api-room-active-status" in ids
    end

    test "offset skips results", %{conn: conn} do
      for i <- 1..3, do: create_room(%{id: "api-offset-room-#{i}", project: "offset-proj"})

      conn_all =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/list_rooms", %{
          "project" => "offset-proj",
          "offset" => "0"
        })

      conn_offset =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/list_rooms", %{
          "project" => "offset-proj",
          "offset" => "2"
        })

      %{"results" => all_results} = json_response(conn_all, 200)
      %{"results" => offset_results} = json_response(conn_offset, 200)
      assert length(all_results) == 3
      assert length(offset_results) == 1
    end

    test "parse_offset clamps negative to 0", %{conn: conn} do
      create_room(%{id: "api-offset-neg", project: "offset-neg-proj"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/list_rooms", %{
          "project" => "offset-neg-proj",
          "offset" => "-5"
        })

      %{"results" => results} = json_response(conn, 200)
      assert length(results) == 1
    end
  end

  describe "POST /api/internal/cluster/room_stats" do
    test "returns stats as JSON", %{conn: conn} do
      room = create_room(%{id: "api-stats"})
      create_message(%{room_id: room.id, author: "Claude"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/room_stats", %{"room_id" => "api-stats"})

      assert %{"results" => results, "warnings" => []} = json_response(conn, 200)
      assert results["room_id"] == "api-stats"
      assert results["message_count"] == 1
    end

    test "returns 400 when room_id missing", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/room_stats", %{})

      assert %{"error" => "room_id is required"} = json_response(conn, 400)
    end

    test "returns null results for nonexistent room", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/room_stats", %{"room_id" => "nonexistent"})

      assert %{"results" => nil} = json_response(conn, 200)
    end

    test "stats include all expected fields", %{conn: conn} do
      room = create_room(%{id: "api-stats-full"})
      create_message(%{room_id: room.id, author: "Claude", message_type: "thought"})
      create_message(%{room_id: room.id, author: "Gemini", message_type: "decision"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/room_stats", %{"room_id" => "api-stats-full"})

      %{"results" => stats} = json_response(conn, 200)
      assert stats["room_id"] == "api-stats-full"
      assert stats["status"] == "active"
      assert stats["message_count"] == 2
      assert stats["participants"] == %{"Claude" => 1, "Gemini" => 1}
      assert stats["type_counts"] == %{"thought" => 1, "decision" => 1}
      assert stats["first_message"] != nil
      assert stats["last_message"] != nil
      assert stats["latest_message_id"] != nil
      assert stats["source_node"] != nil
    end
  end

  describe "POST /api/internal/cluster/get_messages" do
    test "returns messages by message_ids", %{conn: conn} do
      room = create_room(%{id: "api-getmsg"})
      msg1 = create_message(%{room_id: room.id, author: "Claude", content: "first"})
      msg2 = create_message(%{room_id: room.id, author: "Gemini", content: "second"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/get_messages", %{
          "message_ids" => "#{msg1.id},#{msg2.id}"
        })

      assert %{"results" => results, "warnings" => []} = json_response(conn, 200)
      assert length(results) == 2
      contents = Enum.map(results, & &1["content"])
      assert "first" in contents
      assert "second" in contents
    end

    test "returns messages by room_id and last_n", %{conn: conn} do
      room = create_room(%{id: "api-getmsg-room"})
      for i <- 1..5, do: create_message(%{room_id: room.id, content: "msg #{i}"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/get_messages", %{
          "room_id" => "api-getmsg-room",
          "last_n" => "3"
        })

      assert %{"results" => results} = json_response(conn, 200)
      assert length(results) == 3
    end

    test "returns empty results for empty message_ids string", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/get_messages", %{"message_ids" => ""})

      assert %{"results" => []} = json_response(conn, 200)
    end

    # Z2: delta read by room_id + after_id must only return messages after the cursor.
    test "returns messages after a cursor with after_id", %{conn: conn} do
      room = create_room(%{id: "api-getmsg-after"})
      m1 = create_message(%{room_id: room.id, content: "before cursor"})
      _m2 = create_message(%{room_id: room.id, content: "after cursor one"})
      _m3 = create_message(%{room_id: room.id, content: "after cursor two"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/get_messages", %{
          "room_id" => "api-getmsg-after",
          "after_id" => m1.id
        })

      assert %{"results" => results} = json_response(conn, 200)
      contents = Enum.map(results, & &1["content"])
      assert "after cursor one" in contents
      assert "after cursor two" in contents
      refute "before cursor" in contents
    end

    test "message results include all expected fields", %{conn: conn} do
      room = create_room(%{id: "api-getmsg-fields"})
      msg = create_message(%{room_id: room.id, author: "Claude", content: "field check"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/get_messages", %{
          "message_ids" => msg.id
        })

      %{"results" => [result]} = json_response(conn, 200)
      assert result["id"] == msg.id
      assert result["room_id"] == "api-getmsg-fields"
      assert result["author"] == "Claude"
      assert result["content"] == "field check"
      assert result["timestamp"] != nil
    end
  end

  describe "POST /api/internal/cluster/get_digest" do
    test "returns digest for a project", %{conn: conn} do
      room = create_room(%{id: "api-digest", project: "digest-proj"})
      create_message(%{room_id: room.id, author: "Claude", content: "recent work"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/get_digest", %{"project" => "digest-proj"})

      assert %{"warnings" => []} = json_response(conn, 200)
    end

    test "returns empty results for unknown project", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/get_digest", %{"project" => "nonexistent-zzz"})

      assert %{"results" => results} = json_response(conn, 200)
      assert results == [] or is_nil(results)
    end

    test "accepts since parameter", %{conn: conn} do
      room = create_room(%{id: "api-digest-since"})
      create_message(%{room_id: room.id, author: "Claude", content: "old message"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/get_digest", %{
          "project" => "",
          "since" => "2020-01-01T00:00:00"
        })

      assert %{"results" => _results} = json_response(conn, 200)
    end

    test "returns digest with empty params", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/get_digest", %{})

      assert json_response(conn, 200)
    end
  end

  describe "POST /api/internal/cluster/read_transcript" do
    test "returns transcript as JSON", %{conn: conn} do
      room = create_room(%{id: "api-transcript"})
      create_message(%{room_id: room.id, author: "Claude", content: "hello"})
      create_message(%{room_id: room.id, author: "Gemini", content: "hi", pinned: true})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/read_transcript", %{"room_id" => "api-transcript"})

      assert %{"results" => results, "warnings" => []} = json_response(conn, 200)
      assert results["room"]["id"] == "api-transcript"
      assert length(results["messages"]) == 2
      assert results["pinned"]["content"] == "hi"
    end

    test "returns 400 when room_id missing", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/read_transcript", %{})

      assert %{"error" => "room_id is required"} = json_response(conn, 400)
    end

    test "returns null results for nonexistent room", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/read_transcript", %{"room_id" => "nonexistent"})

      assert %{"results" => nil} = json_response(conn, 200)
    end
  end

  describe "POST /api/internal/cluster/locate_room" do
    test "returns the owning node for a public room", %{conn: conn} do
      create_room(%{id: "locate-pub", visibility: "public"})
      create_message(%{room_id: "locate-pub", content: "exists"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/locate_room", %{"room_id" => "locate-pub"})

      assert %{"nodes" => nodes} = json_response(conn, 200)
      assert Atom.to_string(Node.self()) in nodes
    end

    test "does not locate a private room", %{conn: conn} do
      create_room(%{id: "locate-priv", visibility: "private"})
      create_message(%{room_id: "locate-priv", content: "hidden"})

      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/locate_room", %{"room_id" => "locate-priv"})

      assert %{"nodes" => []} = json_response(conn, 200)
    end

    test "returns empty nodes for unknown room", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/locate_room", %{"room_id" => "ghost-room"})

      assert %{"nodes" => []} = json_response(conn, 200)
    end

    test "returns 400 when room_id missing", %{conn: conn} do
      conn =
        conn
        |> put_req_header("content-type", "application/json")
        |> post("/api/internal/cluster/locate_room", %{})

      assert %{"error" => "room_id is required"} = json_response(conn, 400)
    end
  end
end
