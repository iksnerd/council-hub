defmodule CouncilHubUi.ClusterMixedVersionTest do
  # A cluster is mixed-version during every rolling upgrade: the peer may be the
  # previous release and lack a function this release added. The fake peer here
  # answers the way an old node does over :erpc — `{:error, {:exception, :undef, _}}`.
  use CouncilHubUi.DataCase

  import CouncilHubUi.CouncilFixtures

  alias CouncilHubUi.Cluster

  @peer :"council_hub@10.0.0.9"

  defp undef(func) do
    {:error, {:exception, :undef, [{CouncilHubUi.Council, func, [%{"limit" => 4}], []}]}}
  end

  # Local node really runs the query; the peer is scripted by `peer`
  # (func -> erpc reply). `fallback` answers the retry with the old function.
  defp opts(peer, fallback \\ fn _, _ -> {:ok, []} end) do
    [
      nodes: [Node.self(), @peer],
      multicall: fn nodes, Cluster, :local_query, [func, args] ->
        Enum.map(nodes, fn
          @peer -> peer.(func)
          _ -> {:ok, Cluster.local_query(func, args)}
        end)
      end,
      call: fn @peer, func, args -> fallback.(func, args) end,
      version: fn @peer -> "0.63.1" end
    ]
  end

  defp old_room(id) do
    %{
      id: id,
      topic: "old room",
      status: "active",
      visibility: "public",
      project: "p",
      updated_at: ~N[2026-01-01 00:00:00],
      created_at: ~N[2026-01-01 00:00:00]
    }
  end

  describe "list_rooms/2 against a peer without list_rooms_for_cluster" do
    test "retries that peer with the old query and keeps its rooms" do
      create_room(%{id: "mine", project: "p"})

      o =
        opts(
          fn f -> undef(f) end,
          fn :list_rooms_filtered, _ -> {:ok, [old_room("theirs")]} end
        )

      result = Cluster.list_rooms(%{"project" => "p"}, o)

      assert Enum.sort(Enum.map(result.results, & &1.id)) == ["mine", "theirs"]
      theirs = Enum.find(result.results, &(&1.id == "theirs"))
      refute Map.has_key?(theirs, :message_count)

      assert [warning] = result.warnings
      assert warning =~ to_string(@peer)
      assert warning =~ "older version (v0.63.1)"
      assert warning =~ "list_rooms_for_cluster"
      assert warning =~ "no message counts or pinned excerpts"
      refute warning =~ "exception"
      refute warning =~ "undef"
    end

    test "a healthy peer is not retried and adds no warning" do
      o =
        opts(
          fn :list_rooms_for_cluster -> {:ok, [Map.put(old_room("fine"), :message_count, 3)]} end,
          fn _, _ -> flunk("must not retry a healthy peer") end
        )

      result = Cluster.list_rooms(%{}, o)
      assert Enum.any?(result.results, &(&1.id == "fine" and &1.message_count == 3))
      assert result.warnings == []
    end

    test "if the retry fails too, one readable warning and no rooms from that peer" do
      o = opts(fn f -> undef(f) end, fn _, _ -> {:error, {:erpc, :noconnection}} end)
      result = Cluster.list_rooms(%{}, o)
      assert [warning] = result.warnings
      assert warning =~ to_string(@peer)
      refute warning =~ "undef"
    end
  end

  describe "list_room_refs/2 against a peer without list_room_refs" do
    test "falls back to the old room query and says the count may be low" do
      create_room(%{id: "r-mine", project: "p", status: "resolved"})

      o =
        opts(
          fn f -> undef(f) end,
          fn :list_rooms_filtered, args ->
            # the fallback must ask for the most rooms the old query allows
            assert [%{"limit" => 100}] = args
            {:ok, [Map.put(old_room("r-theirs"), :status, "resolved")]}
          end
        )

      result = Cluster.list_room_refs(%{"project" => "p", "status" => "resolved"}, o)

      assert Enum.sort(Enum.map(result.results, & &1.id)) == ["r-mine", "r-theirs"]

      assert Enum.all?(
               result.results,
               &((Map.keys(&1) -- [:source_node]) |> Enum.sort() == [:id, :status, :visibility])
             )

      assert [warning] = result.warnings
      assert warning =~ "older version"
      assert warning =~ "count may be low"
    end
  end

  describe "other errors read as one line" do
    test "an unknown function missing on the peer" do
      o =
        opts(fn _ ->
          {:error, {:exception, :undef, [{CouncilHubUi.Council, :brand_new, [], []}]}}
        end)

      result = Cluster.get_mentions(%{"author" => "x"}, o)
      assert [warning] = result.warnings
      assert warning =~ "older version"
      assert warning =~ "brand_new"
      refute warning =~ "{:exception"
    end

    test "unreachable and timed-out peers" do
      for {reason, text} <- [
            {{:erpc, :noconnection}, "unreachable"},
            {{:erpc, :timeout}, "timed out"}
          ] do
        result = Cluster.get_mentions(%{"author" => "x"}, opts(fn _ -> {:error, reason} end))
        assert [warning] = result.warnings
        assert warning =~ text
        refute warning =~ "{:erpc"
      end
    end
  end

  describe "get_digest/2 against a peer without decision/synthesis counts" do
    test "leaves the counts nil instead of reporting 0" do
      peer_row = %{
        room_id: "d-theirs",
        new_message_count: 2,
        latest_message_excerpt: "x",
        latest_message_id: "i",
        latest_author: "a",
        tags: "",
        posted_from: []
      }

      result = Cluster.get_digest(%{"project" => "p"}, opts(fn _ -> {:ok, [peer_row]} end))
      row = Enum.find(result.results, &(&1.room_id == "d-theirs"))
      assert row.decision_count == nil
      assert row.synthesis_count == nil
    end
  end

  describe "search_messages/2 with work_item against a peer that ignores it" do
    test "filters the peer's unfiltered answer and warns about it" do
      hit = %{id: "1", content: "ship {wi:AB-1} now", timestamp: ~N[2026-01-01 00:00:00]}
      miss = %{id: "2", content: "unrelated", timestamp: ~N[2026-01-02 00:00:00]}

      o =
        opts(fn :search_messages -> {:ok, [hit, miss]} end)
        |> Keyword.put(:multicall, fn nodes, mod, fun, args ->
          case {mod, fun} do
            {Cluster, :supports?} ->
              Enum.map(nodes, fn _ -> undef(:supports?) end)

            _ ->
              Enum.map(nodes, fn
                @peer -> {:ok, [hit, miss]}
                _ -> {:ok, []}
              end)
              |> tap(fn _ -> args end)
          end
        end)

      result = Cluster.search_messages(%{"work_item" => "AB-1", "limit" => 20}, o)
      assert Enum.map(result.results, & &1.id) == ["1"]
      assert [warning] = result.warnings
      assert warning =~ "older version"
      assert warning =~ "work_item"
    end
  end
end
