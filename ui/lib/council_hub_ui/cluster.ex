defmodule CouncilHubUi.Cluster do
  @moduledoc """
  Cluster-wide query fan-out using :erpc.multicall.
  Queries all connected Erlang nodes and aggregates results.
  """

  import Ecto.Query
  require Logger

  alias CouncilHubUi.Council
  alias CouncilHubUi.Council.Room
  alias CouncilHubUi.Repo

  @rpc_timeout 2_000

  @doc """
  Search messages across all cluster nodes.
  Returns %{results: [message_maps], warnings: [strings]}.
  """
  def search_messages(params, opts \\ []) do
    global_limit = Map.get(params, "limit", 20)

    {results, warnings} =
      fan_out(:search_messages, [params], opts)

    {results, warnings} = enforce_work_item(results, warnings, params, opts)

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
  def list_rooms(params, opts \\ []) do
    {results, warnings} =
      fan_out(:list_rooms_for_cluster, [params], opts)

    merged =
      results
      |> List.flatten()
      |> Enum.group_by(& &1.id)
      |> Enum.map(fn {_id, items} -> Enum.max_by(items, & &1.updated_at, NaiveDateTime) end)
      |> Enum.sort_by(& &1.updated_at, {:desc, NaiveDateTime})

    %{results: merged, warnings: warnings}
  end

  @doc """
  Id/status of every room matching the list filters on every node, deduplicated
  by id. Returns %{results: [ref_maps], warnings: [strings]}.
  """
  def list_room_refs(params, opts \\ []) do
    {results, warnings} = fan_out(:list_room_refs, [params], opts)

    merged = results |> List.flatten() |> Enum.uniq_by(& &1.id)
    %{results: merged, warnings: warnings}
  end

  @doc """
  Get room stats across all cluster nodes.
  Returns the first successful result (room typically exists on one node).
  Returns %{results: stats_map | nil, warnings: [strings]}.
  """
  def room_stats(room_id, opts \\ []) do
    {results, warnings} =
      fan_out(:room_stats, [room_id], opts)

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
  def read_transcript(room_id, opts \\ []) do
    {results, warnings} =
      fan_out(:get_room_with_messages, [room_id], opts)

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
  def locate_room(room_id, opts \\ []) do
    {results, warnings} = fan_out(:room_stats, [room_id], opts)

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
  def get_messages(params, opts \\ []) do
    {func, args} =
      cond do
        Map.has_key?(params, "message_ids") ->
          {:get_messages_by_ids, [Map.get(params, "message_ids")]}

        Map.has_key?(params, "after_id") ->
          {:get_messages_since, [Map.get(params, "room_id"), Map.get(params, "after_id")]}

        true ->
          {:get_recent_messages, [Map.get(params, "room_id"), Map.get(params, "limit", 10)]}
      end

    {results, warnings} = fan_out(func, args, opts)

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
  def read_notebook(params, opts \\ []) do
    global_limit = Map.get(params, "limit", 100)

    {results, warnings} = fan_out(:notebook_entries, [params], opts)

    merged =
      results
      |> List.flatten()
      |> Enum.uniq_by(& &1.id)
      |> Enum.sort_by(& &1.id)
      |> Enum.take(-global_limit)

    %{results: merged, warnings: warnings}
  end

  @doc """
  Messages mentioning an agent across all cluster nodes, newest first.
  Returns %{results: [message_maps], warnings: [strings]}.
  """
  def get_mentions(params, opts \\ []) do
    limit = Map.get(params, "limit", 20)

    {results, warnings} =
      fan_out(
        :get_mentions,
        [
          Map.get(params, "author", ""),
          limit,
          Map.get(params, "project", "")
        ],
        opts
      )

    merged =
      results
      |> List.flatten()
      |> Enum.uniq_by(& &1.id)
      |> Enum.sort_by(& &1.id, :desc)
      |> Enum.take(limit)

    %{results: merged, warnings: warnings}
  end

  @doc """
  Get project digest across all cluster nodes.
  Returns %{results: [digest_maps], warnings: [strings]}.
  """
  def get_digest(params, opts \\ []) do
    {results, warnings} =
      fan_out(
        :get_project_digest,
        [Map.get(params, "project", ""), Map.get(params, "since", "")],
        opts
      )

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
          posted_from:
            items |> Enum.flat_map(&Map.get(&1, :posted_from, [])) |> Enum.uniq() |> Enum.sort(),
          latest_message_id: Map.get(best, :latest_message_id, ""),
          latest_author: Map.get(best, :latest_author, ""),
          tags: Map.get(best, :tags, ""),
          # nil, not 0, when the best node predates the field: the consumer
          # tells "no decisions" from "this node cannot count".
          decision_count: Map.get(best, :decision_count),
          synthesis_count: Map.get(best, :synthesis_count),
          source_node: best.source_node
        }
      end)
      |> Enum.sort_by(& &1.new_message_count, :desc)

    %{results: merged, warnings: warnings}
  end

  @doc false
  # Capability probe: peers that predate a feature answer :undef to this very
  # call, which is how a cross-node feature learns the peer cannot do it.
  def supports?(feature), do: feature in [:work_item]

  # Functions added after v0.63.1, with the previous-release call to retry a
  # node with when it answers :undef. `degraded` says what that node's rows
  # lack; `post` reshapes the old function's rows into the new one's shape.
  @fallbacks %{
    list_rooms_for_cluster: %{
      func: :list_rooms_filtered,
      args: :same,
      post: :none,
      degraded: "no message counts or pinned excerpts for its rooms"
    },
    list_room_refs: %{
      func: :list_rooms_filtered,
      args: :max_limit,
      post: :refs,
      degraded: "resolved-room count may be low (counted from at most 100 of its rooms)"
    }
  }

  # A search whose `work_item` an older peer ignores returns that peer's
  # unfiltered top-N. Filter it here (exact {wi:ID} token, as the new query
  # does) and say the peer's list may be short.
  defp enforce_work_item(results, warnings, params, opts) do
    case Map.get(params, "work_item") do
      id when is_binary(id) and id != "" ->
        token = "{wi:" <> id <> "}"
        peers = Keyword.get(opts, :nodes, [Node.self() | Node.list()]) -- [Node.self()]

        old =
          if peers == [] do
            []
          else
            multicall = Keyword.get(opts, :multicall, &default_multicall/4)

            Enum.zip(peers, multicall.(peers, __MODULE__, :supports?, [:work_item]))
            |> Enum.flat_map(fn
              {_node, {:ok, true}} -> []
              {node, _} -> [node]
            end)
          end

        filtered =
          Enum.map(results, fn rows ->
            Enum.filter(rows, fn row ->
              String.contains?(Map.get(row, :content, ""), token) or
                Map.get(row, :source_node) not in Enum.map(old, &Atom.to_string/1)
            end)
          end)

        notes =
          Enum.map(old, fn node ->
            "#{node} runs an older version#{version_suffix(node, opts)} without the work_item filter; " <>
              "its matches were filtered here and may be incomplete"
          end)

        {filtered, warnings ++ notes}

      _ ->
        {results, warnings}
    end
  end

  defp default_multicall(nodes, mod, fun, args),
    do: :erpc.multicall(nodes, mod, fun, args, @rpc_timeout)

  defp default_call(node, func, args) do
    {:ok, :erpc.call(node, __MODULE__, :local_query, [func, args], @rpc_timeout)}
  catch
    :error, reason -> {:error, reason}
    kind, reason -> {:error, {kind, reason}}
  end

  defp default_version(node) do
    case :erpc.call(node, Application, :spec, [:council_hub_ui, :vsn], 1_000) do
      vsn when is_list(vsn) -> to_string(vsn)
      _ -> nil
    end
  catch
    _, _ -> nil
  end

  defp version_suffix(node, opts) do
    case Keyword.get(opts, :version, &default_version/1).(node) do
      nil -> ""
      "v" <> _ = v -> " (#{v})"
      v -> " (v#{v})"
    end
  end

  # Fan out a Council function call to all nodes in the cluster.
  #
  # opts (all optional, for tests): :nodes, :multicall (nodes, mod, fun, args ->
  # replies), :call (node, func, args -> {:ok, v} | {:error, e}, used for the
  # old-function retry), :version (node -> "0.63.1" | nil).
  #
  # Returns {[tagged_results], [warning_strings]}. One warning per node at most,
  # in plain words; the raw error goes to the log, not the tool reply.
  defp fan_out(func, args, opts) do
    nodes = Keyword.get(opts, :nodes, [Node.self() | Node.list()])
    multicall = Keyword.get(opts, :multicall, &default_multicall/4)

    replies = multicall.(nodes, __MODULE__, :local_query, [func, args])

    {tagged_results, warnings} =
      Enum.zip(nodes, replies)
      |> Enum.map(fn {node, reply} -> recover(node, reply, func, args, opts) end)
      |> Enum.reduce({[], []}, fn {node, reply, notes}, {results, warns} ->
        {more, w} = classify(node, reply)
        {more ++ results, Enum.reverse(notes ++ w) ++ warns}
      end)

    {Enum.reverse(tagged_results), Enum.reverse(warnings)}
  end

  # A node that answers :undef for a function added after the previous release
  # is retried with that release's function so its data still appears. Returns
  # {node, reply, notes}; notes explain what that node could not provide.
  defp recover(
         node,
         {:error, {:exception, :undef, [{_mod, missing, _, _} | _]}} = reply,
         func,
         args,
         opts
       ) do
    Logger.warning("cluster: #{node} answered undef for #{inspect(missing)}: #{inspect(reply)}")

    case Map.get(@fallbacks, func) do
      %{func: old_func} = fb when missing == func ->
        call = Keyword.get(opts, :call, &default_call/3)
        old_args = fallback_args(fb.args, args)

        case call.(node, old_func, old_args) do
          {:ok, rows} when is_list(rows) ->
            note =
              "#{node} runs an older version#{version_suffix(node, opts)} without #{func}; " <>
                fb.degraded

            {node, {:ok, shape_fallback(fb.post, rows)}, [note]}

          other ->
            Logger.warning("cluster: #{node} fallback #{old_func} failed: #{inspect(other)}")
            {node, reply, []}
        end

      _ ->
        {node, reply, []}
    end
  end

  defp recover(node, reply, _func, _args, _opts), do: {node, reply, []}

  defp fallback_args(:same, args), do: args

  defp fallback_args(:max_limit, [params | rest]),
    do: [Map.merge(params, %{"limit" => 100, "offset" => 0}) | rest]

  defp shape_fallback(:none, rows), do: rows

  defp shape_fallback(:refs, rows),
    do: Enum.map(rows, &%{id: &1.id, status: &1.status, visibility: &1.visibility})

  # classify/2 turns one node's reply into {tagged_results, warnings}.
  defp classify(node, {:ok, data}) when is_list(data), do: {[tag_with_node(data, node)], []}

  defp classify(node, {:ok, {:ok, data}}) when is_map(data),
    do: {[tag_with_node(data, node)], []}

  defp classify(node, {:ok, {:error, reason}}), do: {[], ["#{node}: #{reason}"]}
  defp classify(node, {:error, reason}), do: {[], [describe_failure(node, reason)]}
  defp classify(node, {:throw, reason}), do: {[], [describe_failure(node, {:throw, reason})]}
  defp classify(node, {:exit, reason}), do: {[], [describe_failure(node, {:exit, reason})]}
  defp classify(node, other), do: {[], [describe_failure(node, {:unexpected, other})]}

  # One readable line per failing node. The raw term is logged, never returned.
  defp describe_failure(node, reason) do
    Logger.warning("cluster: #{node} failed: #{inspect(reason)}")

    case reason do
      {:exception, :undef, [{mod, fun, _, _} | _]} ->
        "#{node} runs an older version without #{inspect(mod)}.#{fun}; its results are missing"

      {:exception, error, _stack} ->
        "#{node} failed while answering (#{error_name(error)}); its results are missing"

      {:erpc, :noconnection} ->
        "#{node} is unreachable; its results are missing"

      {:erpc, :timeout} ->
        "#{node} timed out; its results are missing"

      {:erpc, other} ->
        "#{node} failed (#{other}); its results are missing"

      {:throw, _} ->
        "#{node} failed while answering; its results are missing"

      {:exit, _} ->
        "#{node} dropped the request mid-call; its results are missing"

      _ ->
        "#{node} returned an unexpected reply; its results are missing"
    end
  end

  defp error_name(%{__struct__: mod}), do: inspect(mod)
  defp error_name(atom) when is_atom(atom), do: Atom.to_string(atom)
  defp error_name(_), do: "error"

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
