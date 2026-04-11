defmodule CouncilHubUi.CouncilRooms do
  @moduledoc "Read-only room queries. Called via CouncilHubUi.Council facade."

  import Ecto.Query
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.{Room, Message}

  def list_rooms do
    Repo.all(from r in Room, order_by: [desc: r.updated_at])
  end

  def get_room(id) do
    Repo.get(Room, id)
  end

  def get_room_with_messages(room_id) do
    case get_room(room_id) do
      nil ->
        {:error, "room '#{room_id}' not found"}

      room ->
        messages =
          Repo.all(from m in Message, where: m.room_id == ^room_id, order_by: [asc: m.id])

        pinned =
          Repo.one(from m in Message, where: m.room_id == ^room_id and m.pinned == true, limit: 1)

        {:ok, %{room: room, messages: messages, pinned: pinned}}
    end
  end

  @doc """
  List rooms with optional filters. Mirrors Go ListRooms().
  """
  def list_rooms_filtered(params) when is_map(params) do
    base = from(r in Room, as: :room)

    base =
      case Map.get(params, "project") do
        nil -> base
        "" -> base
        p -> from([room: r] in base, where: r.project == ^p)
      end

    base =
      case Map.get(params, "tag") do
        nil ->
          base

        "" ->
          base

        t ->
          from([room: r] in base,
            where: like(fragment("',' || ? || ','", r.tags), ^"%,#{t},%")
          )
      end

    base =
      case Map.get(params, "status") do
        nil -> base
        "" -> base
        s -> from([room: r] in base, where: r.status == ^s)
      end

    base =
      case Map.get(params, "search") do
        nil ->
          base

        "" ->
          base

        s ->
          words = String.split(s, ~r/\s+/, trim: true)

          Enum.reduce(words, base, fn word, q ->
            pattern = "%#{word}%"

            from([room: r] in q,
              where:
                like(r.id, ^pattern) or
                  like(r.description, ^pattern) or
                  like(r.tags, ^pattern)
            )
          end)
      end

    limit =
      case Map.get(params, "limit") do
        nil -> 50
        "" -> 50
        val when is_integer(val) -> val
        val when is_binary(val) -> String.to_integer(val)
      end

    offset =
      case Map.get(params, "offset") do
        nil -> 0
        "" -> 0
        val when is_integer(val) -> val
        val when is_binary(val) -> String.to_integer(val)
      end

    limit = if limit <= 0, do: 50, else: limit
    limit = if limit > 100, do: 100, else: limit
    offset = if offset < 0, do: 0, else: offset

    Repo.all(
      from [room: r] in base, order_by: [desc: r.updated_at], limit: ^limit, offset: ^offset
    )
  end

  @doc """
  Get room statistics. Mirrors Go GetRoomStats().
  Returns {:ok, stats_map} or {:error, reason}.
  """
  def room_stats(room_id) do
    case get_room(room_id) do
      nil ->
        {:error, "room '#{room_id}' not found"}

      room ->
        stats =
          Repo.one(
            from m in Message,
              where: m.room_id == ^room_id,
              select: %{
                message_count: count(m.id),
                first_message: min(m.timestamp),
                last_message: max(m.timestamp),
                latest_message_id: max(m.id)
              }
          )

        participants =
          Repo.all(
            from m in Message,
              where: m.room_id == ^room_id,
              group_by: m.author,
              select: {m.author, count(m.id)},
              order_by: [desc: count(m.id)]
          )
          |> Map.new()

        type_counts =
          Repo.all(
            from m in Message,
              where: m.room_id == ^room_id and m.is_summary == false,
              group_by: m.message_type,
              select: {m.message_type, count(m.id)},
              order_by: [desc: count(m.id)]
          )
          |> Map.new()

        {:ok,
         %{
           room_id: room_id,
           status: room.status,
           message_count: stats.message_count,
           participants: participants,
           type_counts: type_counts,
           first_message: stats.first_message,
           last_message: stats.last_message,
           latest_message_id: stats.latest_message_id
         }}
    end
  end

  @doc "Returns unique authors for a room as a list of strings."
  def room_participants(room_id) do
    Repo.all(
      from m in Message,
        where: m.room_id == ^room_id and m.is_summary == false,
        group_by: m.author,
        select: m.author
    )
  end

  @doc "Returns [{author, count}] sorted by count desc for a single room."
  def room_participants_with_counts(room_id) do
    Repo.all(
      from m in Message,
        where: m.room_id == ^room_id and m.is_summary == false,
        group_by: m.author,
        select: {m.author, count(m.id)},
        order_by: [desc: count(m.id)]
    )
  end
end
