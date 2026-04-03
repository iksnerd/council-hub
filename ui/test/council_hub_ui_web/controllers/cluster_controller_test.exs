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
end
