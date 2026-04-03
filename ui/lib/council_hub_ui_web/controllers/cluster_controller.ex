defmodule CouncilHubUiWeb.ClusterController do
  use CouncilHubUiWeb, :controller

  alias CouncilHubUi.Cluster

  def search_messages(conn, params) do
    cluster_params = %{
      "query" => Map.get(params, "query", ""),
      "author" => Map.get(params, "author", ""),
      "message_type" => Map.get(params, "message_type", ""),
      "room_id" => Map.get(params, "room_id", ""),
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
      "search" => Map.get(params, "search", "")
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

  def get_messages(conn, params) do
    # Can fetch by message_ids or room_id
    cluster_params =
      if Map.has_key?(params, "message_ids") do
        ids =
          String.split(Map.get(params, "message_ids", ""), ",", trim: true)
          |> Enum.map(&String.trim/1)

        %{"message_ids" => ids}
      else
        %{
          "room_id" => Map.get(params, "room_id", ""),
          "limit" => parse_limit(Map.get(params, "last_n", "10"))
        }
      end

    result = Cluster.get_messages(cluster_params)

    json(conn, %{
      results: Enum.map(result.results, &serialize_message/1),
      warnings: result.warnings
    })
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
