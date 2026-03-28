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
        order_by: [asc: m.id]
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
  def room_participants(room_id) do
    Repo.all(
      from m in Message,
        where: m.room_id == ^room_id and m.is_summary == false,
        distinct: m.author,
        select: m.author
    )
  end
end
