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

  def list_messages_for_room(room_id) do
    Repo.all(
      from m in Message,
        where: m.room_id == ^room_id,
        order_by: [desc: m.pinned, asc: m.id]
    )
  end

  def get_messages_since(room_id, last_id) do
    Repo.all(
      from m in Message,
        where: m.room_id == ^room_id and m.id > ^last_id,
        order_by: [asc: m.id]
    )
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
    reply_tag = if (Map.get(msg, :reply_to, 0) || 0) > 0, do: ", re: ##{msg.reply_to}", else: ""
    ts = format_ts(msg.timestamp)

    cond do
      msg.message_type not in [nil, "", "message"] ->
        "\n**[#{ts}] #{msg.author} (#{msg.message_type}#{reply_tag}):**\n#{msg.content}"

      (Map.get(msg, :reply_to, 0) || 0) > 0 ->
        "\n**[#{ts}] #{msg.author} (re: ##{msg.reply_to}):**\n#{msg.content}"

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
end
