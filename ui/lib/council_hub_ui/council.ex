defmodule CouncilHubUi.Council do
  @moduledoc """
  Read-only context for querying council rooms and messages.
  The Go MCP server owns the schema and writes; we only read.
  """

  import Ecto.Query
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.{Room, Message}

  def list_rooms do
    Repo.all(from r in Room, order_by: [desc: r.updated_at])
  end

  def get_room(id) do
    Repo.get(Room, id)
  end

  def list_messages_for_room(room_id, type_filter \\ "all") do
    base = from m in Message, where: m.room_id == ^room_id

    base =
      if type_filter != "all",
        do: from(m in base, where: m.message_type == ^type_filter or m.is_summary == true),
        else: base

    Repo.all(from m in base, order_by: [desc: m.pinned, asc: m.id])
  end

  def get_messages_since(room_id, last_id, type_filter \\ "all") do
    base = from m in Message, where: m.room_id == ^room_id and m.id > ^last_id

    base =
      if type_filter != "all",
        do: from(m in base, where: m.message_type == ^type_filter or m.is_summary == true),
        else: base

    Repo.all(from m in base, order_by: [asc: m.id])
  end

  def search_messages_in_room(_room_id, query) when query in [nil, ""], do: []

  def search_messages_in_room(room_id, query) do
    q = "%#{String.downcase(query)}%"

    Repo.all(
      from m in Message,
        where:
          m.room_id == ^room_id and
            fragment("lower(?) LIKE ?", m.content, ^q),
        order_by: [asc: m.id],
        limit: 50
    )
  end

  @doc "Returns %{room_id => distinct_author_count} for all rooms."
  def all_room_participant_counts do
    Repo.all(
      from m in Message,
        group_by: m.room_id,
        select: {m.room_id, count(m.author, :distinct)}
    )
    |> Map.new()
  end

  @doc "Returns %{room_id => count} for all rooms in a single query."
  def all_room_message_counts do
    Repo.all(
      from m in Message,
        group_by: m.room_id,
        select: {m.room_id, count(m.id)}
    )
    |> Map.new()
  end

  @doc "Returns the latest updated_at across all rooms, for change detection."
  def latest_room_update do
    Repo.one(from r in Room, select: max(r.updated_at))
  end

  @doc "Returns unique authors for a room as a list of strings."
  def format_transcript(room, messages) do
    header =
      ["# COUNCIL ROOM: #{room.id}"]
      |> maybe_append(room.project, &"**Project:** #{&1}")
      |> maybe_append(room.tech_stack, &"**Tech Stack:** #{&1}")
      |> then(&(&1 ++ ["**Topic:** #{room.description}", "**Status:** #{room.status}"]))
      |> maybe_append(room.tags, &"**Tags:** #{&1}")
      |> maybe_append(Map.get(room, :related_rooms, ""), &"**Related Rooms:** #{&1}")
      |> Enum.join("\n")

    system =
      if present?(room.system_prompt),
        do: "\n*Instructions: #{room.system_prompt}*\n---",
        else: ""

    body =
      messages
      |> Enum.map(&format_message/1)
      |> Enum.join("\n")

    footer = "\n---\n*SYSTEM: You are reading the Council log for \"#{room.id}\". Do not repeat previous points. Use `post_to_room` to contribute your next action.*"

    "#{header}\n---#{system}\n#{body}\n#{footer}\n"
  end

  defp format_message(%{is_summary: true} = msg) do
    "\n**[#{format_ts(msg.timestamp)}] SUMMARY:**\n#{msg.content}"
  end

  defp format_message(msg) do
    reply_to = Map.get(msg, :reply_to, "") || ""
    reply_tag = if reply_to != "", do: ", re: ##{String.slice(reply_to, 0, 8)}", else: ""
    ts = format_ts(msg.timestamp)

    cond do
      msg.message_type not in [nil, "", "message"] ->
        "\n**[#{ts}] #{msg.author} (#{msg.message_type}#{reply_tag}):**\n#{msg.content}"

      reply_to != "" ->
        "\n**[#{ts}] #{msg.author} (re: ##{String.slice(reply_to, 0, 8)}):**\n#{msg.content}"

      true ->
        "\n**[#{ts}] #{msg.author}:**\n#{msg.content}"
    end
  end

  defp format_ts(nil), do: ""
  defp format_ts(dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S")

  defp present?(nil), do: false
  defp present?(""), do: false
  defp present?(_), do: true

  defp maybe_append(lines, value, fmt) do
    if present?(value), do: lines ++ [fmt.(value)], else: lines
  end

  def room_participants(room_id) do
    Repo.all(
      from m in Message,
        where: m.room_id == ^room_id and m.is_summary == false,
        group_by: m.author,
        select: m.author
    )
  end

  @doc """
  Search messages with optional filters. Mirrors Go SearchMessages().
  Accepts a map with optional keys: query, author, message_type, room_id, project, limit.
  """
  def search_messages(params) when is_map(params) do
    limit = Map.get(params, "limit", 20)

    base = from(m in Message, as: :msg)

    base =
      case Map.get(params, "query") do
        nil -> base
        "" -> base
        q -> from([msg: m] in base, where: like(m.content, ^"%#{q}%"))
      end

    base =
      case Map.get(params, "author") do
        nil -> base
        "" -> base
        a -> from([msg: m] in base, where: m.author == ^a)
      end

    base =
      case Map.get(params, "message_type") do
        nil -> base
        "" -> base
        t -> from([msg: m] in base, where: m.message_type == ^t)
      end

    base =
      case Map.get(params, "room_id") do
        nil -> base
        "" -> base
        r -> from([msg: m] in base, where: m.room_id == ^r)
      end

    base =
      case Map.get(params, "project") do
        nil ->
          base

        "" ->
          base

        p ->
          from([msg: m] in base,
            join: r in Room,
            on: m.room_id == r.id,
            where: r.project == ^p
          )
      end

    Repo.all(
      from [msg: m] in base,
        order_by: [desc: m.timestamp],
        limit: ^limit
    )
  end

  @doc """
  List rooms with optional filters. Mirrors Go ListRooms().
  Accepts a map with optional keys: project, tag, status, search.
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
          from([room: r] in base,
            where:
              like(r.id, ^"%#{s}%") or
                like(r.description, ^"%#{s}%") or
                like(r.tags, ^"%#{s}%")
          )
      end

    Repo.all(from [room: r] in base, order_by: [desc: r.updated_at])
  end

  @doc """
  Get room statistics. Mirrors Go GetRoomStats().
  Returns a map with room_id, status, message_count, participants, type_counts,
  first_message, last_message, latest_message_id.
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
end
