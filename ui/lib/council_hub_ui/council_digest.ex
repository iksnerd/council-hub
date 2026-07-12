defmodule CouncilHubUi.CouncilDigest do
  @moduledoc "Project digest query. Called via CouncilHubUi.Council facade."

  import Ecto.Query
  import CouncilHubUi.MessageFilters
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.{Room, Message}
  alias CouncilHubUi.Timestamps

  @doc """
  Get project activity digest. Returns list of %{room_id, message_count, latest_content}.
  Mirror of Go GetProjectDigest.
  """
  def get_project_digest(project, since_str) do
    # Lenient parse (date-only / minute precision accepted); an empty or
    # unparseable `since` falls back to the last 24h, same as the Go side.
    since =
      case Timestamps.parse_since(since_str) do
        nil -> NaiveDateTime.utc_now() |> NaiveDateTime.add(-86400, :second)
        ts -> ts
      end

    base_rooms = from(r in Room, as: :room)

    base_rooms =
      case project do
        nil -> base_rooms
        "" -> base_rooms
        p -> from([room: r] in base_rooms, where: r.project == ^p)
      end

    room_ids = Repo.all(from [room: r] in base_rooms, select: r.id)

    # Counts collapse to live nodes (Go liveClause): an edit's superseded prior
    # versions and retracted tombstones are not "new messages".
    counts =
      from(m in Message,
        where: m.room_id in ^room_ids and m.timestamp > ^since,
        group_by: m.room_id,
        select: {m.room_id, count(m.id)}
      )
      |> live_messages()
      |> Repo.all()
      |> Map.new()

    active_room_ids = for {rid, count} <- counts, count > 0, do: rid

    # Latest per room collapses to the head revision (Go headClause) — a stale
    # prior version never surfaces as "latest"; a retracted head still does, but
    # renders as a tombstone below. UUIDv7 ids order chronologically, so max(id)
    # among heads is the latest head; one IN-query fetches them all (no per-room
    # query loop).
    latest_ids =
      from(m in Message,
        where: m.room_id in ^active_room_ids,
        group_by: m.room_id,
        select: max(m.id)
      )
      |> head_revisions()
      |> Repo.all()

    latest_by_room =
      Repo.all(from m in Message, where: m.id in ^latest_ids)
      |> Map.new(&{&1.room_id, &1})

    for rid <- active_room_ids do
      %{
        room_id: rid,
        new_message_count: Map.fetch!(counts, rid),
        latest_message_excerpt: latest_excerpt(Map.get(latest_by_room, rid))
      }
    end
  end

  defp latest_excerpt(nil), do: ""

  # A retracted latest renders as its tombstone (the Go side masks via
  # DisplayContent), never the withdrawn content.
  defp latest_excerpt(%{retracted_at: %NaiveDateTime{}} = msg) do
    if msg.retracted_by != "" and msg.retracted_by != nil,
      do: "[retracted by #{msg.retracted_by}]",
      else: "[retracted]"
  end

  defp latest_excerpt(%{content: content}), do: excerpt(content)

  defp excerpt(content) do
    if String.contains?(content, "# ") do
      parts = String.split(content, "# ")

      if length(parts) > 1 do
        parts |> Enum.at(1) |> String.split("\n") |> List.first() |> String.trim()
      else
        content
      end
    else
      first_sentence = String.split(content, ". ") |> List.first()

      if first_sentence && String.length(first_sentence) > 120 do
        truncated = String.slice(first_sentence, 0, 120)

        case Regex.run(~r/.*(?=\s)/, truncated) do
          [matched] -> matched <> "..."
          nil -> truncated <> "..."
        end
      else
        (first_sentence || "") <> "..."
      end
    end
  end
end
