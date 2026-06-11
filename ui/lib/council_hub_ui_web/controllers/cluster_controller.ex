defmodule CouncilHubUiWeb.ClusterController do
  use CouncilHubUiWeb, :controller

  alias CouncilHubUi.Cluster

  def nodes(conn, _params) do
    local_vsn = Application.spec(:council_hub_ui, :vsn) |> to_string()
    local = %{node: to_string(Node.self()), version: local_vsn}

    peers =
      Node.list()
      |> Enum.map(fn node ->
        version =
          case :erpc.call(node, Application, :spec, [:council_hub_ui, :vsn], 1_000) do
            vsn when is_list(vsn) -> to_string(vsn)
            _ -> "unknown"
          end

        %{node: to_string(node), version: version}
      end)

    all = [local | peers]

    versions = all |> Enum.map(& &1.version) |> Enum.uniq()
    mismatch = length(versions) > 1

    json(conn, %{nodes: all, count: length(all), version_mismatch: mismatch})
  end

  def search_messages(conn, params) do
    cluster_params = %{
      "query" => Map.get(params, "query", ""),
      "author" => Map.get(params, "author", ""),
      "message_type" => Map.get(params, "message_type", ""),
      "room_id" => Map.get(params, "room_id", ""),
      "room_ids" => Map.get(params, "room_ids", ""),
      "project" => Map.get(params, "project", ""),
      "since" => Map.get(params, "since", ""),
      "until" => Map.get(params, "until", ""),
      "limit" => parse_limit(Map.get(params, "limit", "20"))
    }

    result = Cluster.search_messages(cluster_params)

    json(conn, %{
      results: Enum.map(result.results, &serialize_message/1),
      warnings: result.warnings
    })
  end

  def list_rooms(conn, params) do
    cluster_params = %{
      "project" => Map.get(params, "project", ""),
      "tag" => Map.get(params, "tag", ""),
      "status" => Map.get(params, "status", ""),
      "search" => Map.get(params, "search", ""),
      "limit" => parse_limit(Map.get(params, "limit", "50")),
      "offset" => parse_offset(Map.get(params, "offset", "0"))
    }

    result = Cluster.list_rooms(cluster_params)

    json(conn, %{
      results: Enum.map(result.results, &serialize_room/1),
      warnings: result.warnings
    })
  end

  def room_stats(conn, params) do
    room_id = Map.get(params, "room_id", "")

    if room_id == "" do
      conn |> put_status(400) |> json(%{error: "room_id is required"})
    else
      result = Cluster.room_stats(room_id)

      json(conn, %{
        results: serialize_stats(result.results),
        warnings: result.warnings
      })
    end
  end

  def read_transcript(conn, params) do
    room_id = Map.get(params, "room_id", "")

    if room_id == "" do
      conn |> put_status(400) |> json(%{error: "room_id is required"})
    else
      result = Cluster.read_transcript(room_id)

      if result.results do
        json(conn, %{
          results: %{
            room: serialize_room(result.results.room),
            messages: Enum.map(result.results.messages, &serialize_message/1),
            pinned: serialize_message_optional(result.results.pinned)
          },
          warnings: result.warnings
        })
      else
        json(conn, %{
          results: nil,
          warnings: result.warnings
        })
      end
    end
  end

  def locate_room(conn, params) do
    room_id = Map.get(params, "room_id", "")

    if room_id == "" do
      conn |> put_status(400) |> json(%{error: "room_id is required"})
    else
      result = Cluster.locate_room(room_id)
      json(conn, %{nodes: result.nodes, warnings: result.warnings})
    end
  end

  def get_messages(conn, params) do
    # Three modes: by message_ids, delta read (room_id + after_id), or recent (room_id + last_n).
    # Match on non-empty values, not key presence — the Go server always sends every key
    # (empty string when unset), so Map.has_key? would always pick the message_ids branch.
    message_ids = Map.get(params, "message_ids", "")
    room_id = Map.get(params, "room_id", "")
    after_id = Map.get(params, "after_id", "")

    cluster_params =
      cond do
        message_ids != "" ->
          ids =
            String.split(message_ids, ",", trim: true)
            |> Enum.map(&String.trim/1)

          %{"message_ids" => ids}

        room_id != "" and after_id != "" ->
          %{"room_id" => room_id, "after_id" => after_id}

        true ->
          %{
            "room_id" => room_id,
            "limit" => parse_limit(Map.get(params, "last_n", "10"))
          }
      end

    result = Cluster.get_messages(cluster_params)

    json(conn, %{
      results: Enum.map(result.results, &serialize_message/1),
      warnings: result.warnings
    })
  end

  def read_notebook(conn, params) do
    project = Map.get(params, "project", "")

    if project == "" do
      conn |> put_status(400) |> json(%{error: "project is required"})
    else
      cluster_params = %{
        "project" => project,
        "types" => Map.get(params, "types", ""),
        "since" => Map.get(params, "since", ""),
        "until" => Map.get(params, "until", ""),
        "after_id" => Map.get(params, "after_id", ""),
        "limit" => parse_notebook_limit(Map.get(params, "limit", ""))
      }

      result = Cluster.read_notebook(cluster_params)

      json(conn, %{
        results: Enum.map(result.results, &serialize_notebook_entry/1),
        warnings: result.warnings
      })
    end
  end

  def get_digest(conn, params) do
    cluster_params = %{
      "project" => Map.get(params, "project", ""),
      "since" => Map.get(params, "since", "")
    }

    result = Cluster.get_digest(cluster_params)

    json(conn, %{
      results: result.results,
      warnings: result.warnings
    })
  end

  defp parse_limit(val) when is_binary(val) do
    case Integer.parse(val) do
      {n, _} when n > 0 and n <= 100 -> n
      _ -> 20
    end
  end

  defp parse_limit(val) when is_integer(val), do: min(max(val, 1), 100)
  defp parse_limit(_), do: 20

  defp parse_offset(val) when is_binary(val) do
    case Integer.parse(val) do
      {n, _} when n >= 0 -> n
      _ -> 0
    end
  end

  defp parse_offset(val) when is_integer(val), do: max(val, 0)
  defp parse_offset(_), do: 0

  # Notebook entries cap at 500 (vs 100 elsewhere) — a project timeline
  # legitimately spans more items than a search result page.
  defp parse_notebook_limit(val) when is_binary(val) do
    case Integer.parse(val) do
      {n, _} when n > 0 and n <= 500 -> n
      {n, _} when n > 500 -> 500
      _ -> 100
    end
  end

  defp parse_notebook_limit(val) when is_integer(val), do: min(max(val, 1), 500)
  defp parse_notebook_limit(_), do: 100

  defp serialize_notebook_entry(entry) do
    entry
    |> serialize_message()
    |> Map.put(:repo, Map.get(entry, :repo, ""))
  end

  defp serialize_message_optional(nil), do: nil
  defp serialize_message_optional(msg), do: serialize_message(msg)

  defp serialize_message(msg) do
    %{
      id: msg.id,
      room_id: msg.room_id,
      author: msg.author,
      content: msg.content,
      message_type: msg.message_type,
      is_summary: msg.is_summary,
      reply_to: msg.reply_to,
      pinned: msg.pinned,
      timestamp: format_datetime(msg.timestamp),
      source_node: Map.get(msg, :source_node, nil)
    }
  end

  defp serialize_room(room) do
    %{
      id: room.id,
      description: room.description,
      status: room.status,
      project: room.project,
      tech_stack: room.tech_stack,
      tags: room.tags,
      system_prompt: room.system_prompt,
      related_rooms: room.related_rooms,
      visibility: Map.get(room, :visibility, "public"),
      repo: Map.get(room, :repo, ""),
      created_at: format_datetime(room.created_at),
      updated_at: format_datetime(room.updated_at),
      source_node: Map.get(room, :source_node, nil)
    }
  end

  defp serialize_stats(nil), do: nil

  defp serialize_stats(stats) do
    %{
      room_id: stats.room_id,
      status: stats.status,
      message_count: stats.message_count,
      participants: stats.participants,
      type_counts: stats.type_counts,
      first_message: format_datetime(stats.first_message),
      last_message: format_datetime(stats.last_message),
      latest_message_id: stats.latest_message_id,
      source_node: Map.get(stats, :source_node, nil)
    }
  end

  defp format_datetime(nil), do: nil
  defp format_datetime(%NaiveDateTime{} = dt), do: NaiveDateTime.to_iso8601(dt)
  defp format_datetime(other), do: to_string(other)
end
