defmodule CouncilHubUi.BulkStats do
  @moduledoc "Batch room-level aggregate queries. Called via CouncilHubUi.Council facade."

  import Ecto.Query
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.{Room, Message}

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

  @doc "Returns a MapSet of room IDs that have at least one synthesis message."
  def all_room_synthesis_flags do
    Repo.all(
      from m in Message,
        where: m.message_type == "synthesis",
        distinct: true,
        select: m.room_id
    )
    |> MapSet.new()
  end

  @doc "Returns %{room_id => latest_message_id} for all rooms in a single query."
  def all_room_latest_message_ids do
    Repo.all(
      from m in Message,
        group_by: m.room_id,
        select: {m.room_id, max(m.id)}
    )
    |> Map.new()
  end

  @doc "Returns %{room_id => %{\"decision\" => n, \"action\" => n}} in a single batch query."
  def all_room_key_type_counts do
    Repo.all(
      from m in Message,
        where: m.message_type in ["decision", "action"],
        group_by: [m.room_id, m.message_type],
        select: {m.room_id, m.message_type, count(m.id)}
    )
    |> Enum.reduce(%{}, fn {room_id, type, count}, acc ->
      Map.update(acc, room_id, %{type => count}, &Map.put(&1, type, count))
    end)
  end

  @doc "Returns %{room_id => %{type => count}} for ALL message types in a single batch query."
  def all_room_full_type_counts do
    Repo.all(
      from m in Message,
        where: m.is_summary == false,
        group_by: [m.room_id, m.message_type],
        select: {m.room_id, m.message_type, count(m.id)}
    )
    |> Enum.reduce(%{}, fn {room_id, type, count}, acc ->
      Map.update(acc, room_id, %{type => count}, &Map.put(&1, type, count))
    end)
  end

  @doc "Returns %{room_id => {first_timestamp, last_timestamp}} for all rooms."
  def all_room_time_ranges do
    Repo.all(
      from m in Message,
        group_by: m.room_id,
        select: {m.room_id, min(m.timestamp), max(m.timestamp)}
    )
    |> Map.new(fn {room_id, first, last} -> {room_id, {first, last}} end)
  end

  @doc "Returns the latest updated_at across all rooms, for change detection."
  def latest_room_update do
    Repo.one(from r in Room, select: max(r.updated_at))
  end
end
