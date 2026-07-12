defmodule CouncilHubUi.Cluster do
  @moduledoc """
  Cluster-wide query fan-out using :erpc.multicall.
  Queries all connected Erlang nodes and aggregates results.
  """

  import Ecto.Query

  alias CouncilHubUi.Council
  alias CouncilHubUi.Council.Room
  alias CouncilHubUi.Repo

  @rpc_timeout 2_000

  @doc """
  Search messages across all cluster nodes.
  Returns %{results: [message_maps], warnings: [strings]}.
  """
  def search_messages(params) do
    global_limit = Map.get(params, "limit", 20)

    {results, warnings} =
      fan_out(:search_messages, [params])

    merged =
      results
      |> List.flatten()
      |> Enum.sort_by(& &1.timestamp, {:desc, NaiveDateTime})
      |> Enum.take(global_limit)

    %{results: merged, warnings: warnings}
  end

  @doc """
  List rooms across all cluster nodes.
  Returns %{results: [room_maps], warnings: [strings]}.
  """
  def list_rooms(params) do
    {results, warnings} =
      fan_out(:list_rooms_filtered, [params])

    merged =
      results
      |> List.flatten()
      |> Enum.group_by(& &1.id)
      |> Enum.map(fn {_id, items} -> Enum.max_by(items, & &1.updated_at, NaiveDateTime) end)
      |> Enum.sort_by(& &1.updated_at, {:desc, NaiveDateTime})

    %{results: merged, warnings: warnings}
  end

  @doc """
  Get room stats across all cluster nodes.
  Returns the first successful result (room typically exists on one node).
  Returns %{results: stats_map | nil, warnings: [strings]}.
  """
  def room_stats(room_id) do
    {results, warnings} =
      fan_out(:room_stats, [room_id])

    # fan_out already unwraps {:ok, data} from local_query,
    # and room_stats returns {:ok, map} which fan_out handles via tag_with_node.
    # Results are already tagged maps or nil.
    stats = Enum.max_by(results, fn s -> Map.get(s, :message_count, 0) end, fn -> nil end)

    %{results: stats, warnings: warnings}
  end

  @doc """
  Read transcript data (room, messages, pinned) across all cluster nodes.
  Returns the first successful result (room typically exists on one node).
  Returns %{results: raw_data_map | nil, warnings: [strings]}.
  """
  def read_transcript(room_id) do
    {results, warnings} =
      fan_out(:get_room_with_messages, [room_id])

    # Prefer the node with the most messages — the local node may hold an empty
    # stub while the authoritative copy lives on a peer.
    data = Enum.max_by(results, fn r -> length(Map.get(r, :messages, [])) end, fn -> nil end)

    %{results: data, warnings: warnings}
  end

  @doc """
  Locate the cluster node(s) that own a (public) room. Fans out room_stats and
  collects the source nodes that hold the room. Private rooms are excluded by the
  fan-out gate, so they are never located. Returns %{nodes: [node_names], warnings: [strings]}.
  """
  def locate_room(room_id) do
    {results, warnings} = fan_out(:room_stats, [room_id])

    nodes =
      results
      |> Enum.map(&Map.get(&1, :source_node))
      |> Enum.reject(&is_nil/1)
      |> Enum.uniq()

    %{nodes: nodes, warnings: warnings}
  end

  @doc """
  Fetch messages by IDs or room recent messages across all cluster nodes.
  Returns %{results: [message_maps], warnings: [strings]}.
  """
  def get_messages(params) do
    {func, args} =
      cond do
        Map.has_key?(params, "message_ids") ->
          {:get_messages_by_ids, [Map.get(params, "message_ids")]}

        Map.has_key?(params, "after_id") ->
          {:get_messages_since, [Map.get(params, "room_id"), Map.get(params, "after_id")]}

        true ->
          {:get_recent_messages, [Map.get(params, "room_id"), Map.get(params, "limit", 10)]}
      end

    {results, warnings} = fan_out(func, args)

    merged =
      results
      |> List.flatten()
      |> Enum.uniq_by(& &1.id)
      |> Enum.sort_by(& &1.timestamp, {:asc, NaiveDateTime})

    %{results: merged, warnings: warnings}
  end

  @doc """
  Read a project's notebook timeline across all cluster nodes. Entries merge by
  UUIDv7 message ID — lexicographic order is chronological even across nodes —
  and limit keeps the most recent entries. Returns %{results: [entry_maps], warnings: [strings]}.
  """
  def read_notebook(params) do
    global_limit = Map.get(params, "limit", 100)

    {results, warnings} = fan_out(:notebook_entries, [params])

    merged =
      results
      |> List.flatten()
      |> Enum.uniq_by(& &1.id)
      |> Enum.sort_by(& &1.id)
      |> Enum.take(-global_limit)

    %{results: merged, warnings: warnings}
  end

  @doc """
  Get project digest across all cluster nodes.
  Returns %{results: [digest_maps], warnings: [strings]}.
  """
  def get_digest(params) do
    {results, warnings} =
      fan_out(:get_project_digest, [Map.get(params, "project", ""), Map.get(params, "since", "")])

    merged =
      results
      |> List.flatten()
      # Group by room_id and merge counts (assuming room might exist on multiple nodes, though unlikely)
      |> Enum.group_by(& &1.room_id)
      |> Enum.map(fn {room_id, items} ->
        total_count = Enum.reduce(items, 0, &(&1.new_message_count + &2))
        # Prefer the node that reported the most activity for this room
        best = Enum.max_by(items, & &1.new_message_count)

        %{
          room_id: room_id,
          new_message_count: total_count,
          latest_message_excerpt: best.latest_message_excerpt,
          source_node: best.source_node
        }
      end)
      |> Enum.sort_by(& &1.new_message_count, :desc)

    %{results: merged, warnings: warnings}
  end

  # Fan out a Council function call to all nodes in the cluster.

  # Returns {[tagged_results], [warning_strings]}.
  defp fan_out(func, args) do
    nodes = [Node.self() | Node.list()]

    # :erpc.multicall/5 returns a flat list of {:ok, Result} | {:error, Info}
    replies = :erpc.multicall(nodes, __MODULE__, :local_query, [func, args], @rpc_timeout)

    # erpc wraps successful calls in {:ok, value}.
    # value is the raw return from local_query (list, {:ok, map}, or {:error, msg}).
    {tagged_results, warnings} =
      Enum.zip(nodes, replies)
      |> Enum.reduce({[], []}, fn
        # List results (search_messages, list_rooms_filtered)
        {node, {:ok, data}}, {results, warns} when is_list(data) ->
          {[tag_with_node(data, node) | results], warns}

        # Ok-tuple results (room_stats success, read_transcript success)
        {node, {:ok, {:ok, data}}}, {results, warns} when is_map(data) ->
          {[tag_with_node(data, node) | results], warns}

        # Error-tuple results (room_stats not found)
        {node, {:ok, {:error, reason}}}, {results, warns} ->
          {results, ["#{node}: #{reason}" | warns]}

        # erpc failures (node unreachable, timeout, etc.)
        {node, {:error, reason}}, {results, warns} ->
          {results, ["#{node}: #{inspect(reason)}" | warns]}

        # erpc failures where the remote call itself threw or the remote process
        # exited mid-call — :erpc.multicall/5 can return either shape, and without
        # a clause here a single such peer raised FunctionClauseError and 500'd
        # every cluster_wide query, not just the one bad node.
        {node, {:throw, reason}}, {results, warns} ->
          {results, ["#{node}: threw #{inspect(reason)}" | warns]}

        {node, {:exit, reason}}, {results, warns} ->
          {results, ["#{node}: exited #{inspect(reason)}" | warns]}

        # Anything else unexpected — degrade to a warning rather than crash.
        {node, other}, {results, warns} ->
          {results, ["#{node}: unexpected reply #{inspect(other)}" | warns]}
      end)

    {Enum.reverse(tagged_results), Enum.reverse(warnings)}
  end

  @doc false
  # Called via :erpc on each node. Executes a local Council query, then strips
  # any private rooms (and their messages) from the result so they never leave
  # this node. Private rooms remain fully visible to local/UI queries — only the
  # cluster fan-out path goes through here.
  def local_query(func, args) do
    apply(Council, func, args)
    |> reject_private()
  end

  # Drop private rooms from cluster fan-out results. Shapes returned by the
  # fanned-out Council functions: a list (rooms/messages/digest items), an
  # {:ok, %{room: ...}} tuple (read_transcript), an {:ok, %{room_id: ...}} tuple
  # (room_stats), or an {:error, _} tuple.
  defp reject_private(result) do
    private = private_room_ids()

    case result do
      list when is_list(list) ->
        Enum.reject(list, &private_item?(&1, private))

      {:ok, %{room: %{id: id}}} = ok ->
        if MapSet.member?(private, id), do: {:error, "not found"}, else: ok

      {:ok, %{room_id: rid}} = ok ->
        if MapSet.member?(private, rid), do: {:error, "not found"}, else: ok

      other ->
        other
    end
  end

  # A room result carries its own visibility; a message/digest result carries a
  # room_id whose room may be private.
  defp private_item?(%{visibility: "private"}, _private), do: true
  defp private_item?(%{room_id: rid}, private), do: MapSet.member?(private, rid)
  defp private_item?(_item, _private), do: false

  defp private_room_ids do
    Repo.all(from r in Room, where: r.visibility == "private", select: r.id)
    |> MapSet.new()
  end

  # Tag results with source node name.
  # For lists (messages, rooms), tag each item.
  # For maps (stats), tag the map directly.
  defp tag_with_node(items, node) when is_list(items) do
    Enum.map(items, fn item ->
      Map.put(item, :source_node, Atom.to_string(node))
    end)
  end

  defp tag_with_node(item, node) when is_map(item) do
    Map.put(item, :source_node, Atom.to_string(node))
  end
end
