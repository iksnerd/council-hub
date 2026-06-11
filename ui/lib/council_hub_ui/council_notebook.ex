defmodule CouncilHubUi.CouncilNotebook do
  @moduledoc """
  Read-only project notebook queries: typed messages from every room in a
  project woven into one chronological timeline. Mirrors the Go server's
  GetNotebookEntries (notebook.go) — UUIDv7 message IDs sort lexicographically
  by creation time, so ordering by id merges rooms without a timestamp merge.
  Called via the CouncilHubUi.Council facade (locally and via cluster fan-out).
  """

  import Ecto.Query
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.{Room, Message}

  @default_types ~w(decision action synthesis)
  @default_limit 100
  @max_limit 500

  def default_types, do: @default_types

  @doc """
  Returns notebook entries for a project as maps (message fields + the owning
  room's `repo` for {sha:...} resolution). Accepts a string-keyed params map:
  project (required), types (CSV), since, until, after_id, limit. When limit
  truncates, the most recent entries are kept; output is chronological.
  """
  def notebook_entries(params) when is_map(params) do
    project = Map.get(params, "project", "")

    if project == "" do
      []
    else
      types = parse_types(Map.get(params, "types", ""))
      limit = parse_limit(Map.get(params, "limit"))

      base =
        from m in Message,
          join: r in Room,
          on: m.room_id == r.id,
          where: r.project == ^project and m.is_summary == false and m.message_type in ^types

      base =
        case parse_timestamp(Map.get(params, "since", "")) do
          nil -> base
          since -> from [m, r] in base, where: m.timestamp >= ^since
        end

      base =
        case parse_timestamp(Map.get(params, "until", "")) do
          nil -> base
          until_ts -> from [m, r] in base, where: m.timestamp <= ^until_ts
        end

      base =
        case Map.get(params, "after_id", "") do
          "" -> base
          after_id -> from [m, r] in base, where: m.id > ^after_id
        end

      Repo.all(
        from [m, r] in base,
          order_by: [desc: m.id],
          limit: ^limit,
          select: %{
            id: m.id,
            room_id: m.room_id,
            author: m.author,
            content: m.content,
            message_type: m.message_type,
            is_summary: m.is_summary,
            reply_to: m.reply_to,
            pinned: m.pinned,
            timestamp: m.timestamp,
            repo: r.repo
          }
      )
      |> Enum.reverse()
    end
  end

  @doc "Distinct non-empty project names, alphabetical. Feeds the /notebook picker."
  def list_projects do
    Repo.all(
      from r in Room,
        where: r.project != "",
        distinct: true,
        order_by: r.project,
        select: r.project
    )
  end

  defp parse_types(""), do: @default_types
  defp parse_types(nil), do: @default_types

  defp parse_types(csv) when is_binary(csv) do
    case csv
         |> String.split(",", trim: true)
         |> Enum.map(&String.trim/1)
         |> Enum.reject(&(&1 == "")) do
      [] -> @default_types
      types -> types
    end
  end

  defp parse_timestamp(""), do: nil
  defp parse_timestamp(nil), do: nil

  defp parse_timestamp(str) when is_binary(str) do
    case NaiveDateTime.from_iso8601(str) do
      {:ok, ts} -> ts
      _ -> nil
    end
  end

  defp parse_limit(nil), do: @default_limit
  defp parse_limit(n) when is_integer(n) and n > 0, do: min(n, @max_limit)
  defp parse_limit(n) when is_integer(n), do: @default_limit

  defp parse_limit(str) when is_binary(str) do
    case Integer.parse(str) do
      {n, _} when n > 0 -> min(n, @max_limit)
      _ -> @default_limit
    end
  end

  defp parse_limit(_), do: @default_limit
end
