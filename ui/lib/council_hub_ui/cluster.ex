defmodule CouncilHubUi.Cluster do
  @moduledoc """
  Cluster-wide query fan-out using :erpc.multicall.
  Queries all connected Erlang nodes and aggregates results.
  """

  alias CouncilHubUi.Council

  @rpc_timeout 5_000

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
    stats = List.first(results)

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

    data = List.first(results)

    %{results: data, warnings: warnings}
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
      end)

    {Enum.reverse(tagged_results), Enum.reverse(warnings)}
  end

  @doc false
  # Called via :erpc on each node. Executes a local Council query.
  # Returns the raw result from Council — caller handles ok/error tuples.
  def local_query(func, args) do
    apply(Council, func, args)
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
