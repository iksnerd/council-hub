defmodule CouncilHubUi.CouncilDigest do
  @moduledoc "Project digest query. Called via CouncilHubUi.Council facade."

  import Ecto.Query
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.{Room, Message}

  @doc """
  Get project activity digest. Returns list of %{room_id, message_count, latest_content}.
  Mirror of Go GetProjectDigest.
  """
  def get_project_digest(project, since_str) do
    since =
      case NaiveDateTime.from_iso8601(since_str) do
        {:ok, dt} -> dt
        # fallback 24h
        _ -> NaiveDateTime.utc_now() |> NaiveDateTime.add(-86400, :second)
      end

    base_rooms = from(r in Room, as: :room)

    base_rooms =
      case project do
        nil -> base_rooms
        "" -> base_rooms
        p -> from([room: r] in base_rooms, where: r.project == ^p)
      end

    rooms = Repo.all(from [room: r] in base_rooms, select: {r.id, r.project})

    Enum.reduce(rooms, [], fn {rid, _}, acc ->
      count =
        Repo.one(
          from m in Message,
            where: m.room_id == ^rid and m.timestamp > ^since,
            select: count(m.id)
        )

      if count > 0 do
        latest =
          Repo.one(
            from m in Message, where: m.room_id == ^rid, order_by: [desc: m.timestamp], limit: 1
          )

        content = if latest, do: latest.content, else: ""

        excerpt = content

        excerpt =
          if String.contains?(excerpt, "# ") do
            parts = String.split(excerpt, "# ")

            if length(parts) > 1 do
              parts |> Enum.at(1) |> String.split("\n") |> List.first() |> String.trim()
            else
              excerpt
            end
          else
            first_sentence = String.split(excerpt, ". ") |> List.first()

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

        item = %{room_id: rid, new_message_count: count, latest_message_excerpt: excerpt}
        [item | acc]
      else
        acc
      end
    end)
  end
end
