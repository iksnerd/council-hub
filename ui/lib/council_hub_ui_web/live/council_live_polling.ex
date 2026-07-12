defmodule CouncilHubUiWeb.CouncilLivePolling do
  @moduledoc """
  Pure polling and loading helpers extracted from CouncilLive.
  All functions accept and return Phoenix.LiveView sockets or plain values.
  """

  import Phoenix.Component, only: [assign: 2]
  import Phoenix.LiveView, only: [stream: 4, stream_insert: 3, stream_delete: 3]
  alias CouncilHubUi.{Council, Cluster}
  require Logger

  # A poll tick only re-checks this many of the most-recently-seen messages for
  # in-place mutations (reactions/pins/retractions/edits) — bounded so the check
  # stays cheap on rooms with long histories. UUIDv7 ids sort chronologically.
  @mutation_window 300

  # -- Schedule --

  def schedule_poll(msg, interval), do: Process.send_after(self(), msg, interval)

  # -- Message polling --

  def poll_active_room_messages(%{assigns: %{active_room: nil}} = socket), do: socket

  def poll_active_room_messages(
        %{
          assigns: %{
            active_room: room,
            last_msg_id: last_id,
            type_filter: type_filter,
            searching: searching
          }
        } = socket
      ) do
    case Council.get_messages_since(room.id, last_id, type_filter) do
      [] ->
        socket

      new_messages ->
        new_last_id = last_message_id(new_messages)

        if searching do
          assign(socket, last_msg_id: new_last_id)
        else
          socket
          |> stream(:messages, new_messages, at: -1)
          |> assign(
            last_msg_id: new_last_id,
            has_messages: true,
            msg_state: Map.merge(socket.assigns.msg_state, build_msg_state(new_messages))
          )
        end
    end
  end

  # -- In-place mutation polling --
  #
  # New-message polling (above) only ever appends rows past last_msg_id, so a
  # reaction, pin, or retraction on an already-displayed message — an UPDATE, not
  # an INSERT — never reaches a connected viewer, and an edit (which appends a
  # new head and flags the old row `revised`) leaves the stale row on screen
  # forever instead of being replaced. Re-checking a bounded window of already-seen
  # ids each tick catches all three.

  @doc "Snapshot of a message's mutable fields, keyed by id — used to diff polls."
  def build_msg_state(messages), do: Map.new(messages, &{&1.id, &1})

  def poll_active_room_mutations(%{assigns: %{active_room: nil}} = socket), do: socket
  def poll_active_room_mutations(%{assigns: %{searching: true}} = socket), do: socket

  def poll_active_room_mutations(%{assigns: %{msg_state: msg_state}} = socket)
      when map_size(msg_state) == 0,
      do: socket

  def poll_active_room_mutations(%{assigns: %{msg_state: msg_state}} = socket) do
    ids = msg_state |> Map.keys() |> Enum.sort(:desc) |> Enum.take(@mutation_window)

    # Slim fetch — the diff below only reads the small mutable fields, so don't
    # pull up to @mutation_window full content rows every 3s tick per socket.
    ids
    |> Council.get_messages_mutable_state()
    |> Enum.reduce(socket, &apply_mutation(&2, &1))
  end

  defp apply_mutation(socket, fresh) do
    case Map.get(socket.assigns.msg_state, fresh.id) do
      nil ->
        socket

      old ->
        cond do
          fresh.revised ->
            socket
            |> stream_delete(:messages, old)
            |> assign(msg_state: Map.delete(socket.assigns.msg_state, fresh.id))

          mutated?(old, fresh) ->
            merged = %{
              old
              | pinned: fresh.pinned,
                reactions: fresh.reactions,
                retracted_at: fresh.retracted_at,
                retracted_by: fresh.retracted_by
            }

            socket
            |> stream_insert(:messages, merged)
            |> assign(msg_state: Map.put(socket.assigns.msg_state, fresh.id, merged))

          true ->
            socket
        end
    end
  end

  defp mutated?(old, fresh) do
    old.pinned != fresh.pinned or old.reactions != fresh.reactions or
      old.retracted_at != fresh.retracted_at or old.retracted_by != fresh.retracted_by
  end

  # -- Room polling --

  def poll_rooms_full(%{assigns: %{cluster_wide: true}} = socket, _latest) do
    {rooms, warnings} = load_cluster_rooms()

    {:noreply,
     assign(socket,
       rooms: rooms,
       rooms_by_project: group_rooms_by_project(rooms),
       cluster_warnings: warnings
     )}
  end

  def poll_rooms_full(socket, latest) do
    {rooms, db_connected} = load_rooms()
    new_counts = safe_room_counts(db_connected)
    new_participants = safe_participant_counts(db_connected)
    new_latest_ids = safe_latest_ids(db_connected)

    socket =
      assign(socket,
        rooms: rooms,
        rooms_by_project: group_rooms_by_project(rooms),
        room_counts: new_counts,
        participant_counts: new_participants,
        latest_ids: new_latest_ids,
        synthesis_flags: safe_synthesis_flags(db_connected),
        type_counts: safe_type_counts(db_connected),
        time_ranges: safe_time_ranges(db_connected),
        db_connected: db_connected,
        last_room_update: latest
      )

    # Update active room status if changed
    socket =
      case socket.assigns.active_room do
        nil ->
          socket

        active ->
          case Enum.find(rooms, &(&1.id == active.id)) do
            nil -> socket
            updated when updated.status != active.status -> assign(socket, active_room: updated)
            _ -> socket
          end
      end

    {:noreply, socket}
  end

  # -- Loaders --

  def load_rooms do
    {Council.list_rooms(), true}
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load rooms: #{inspect(e)}")
      {[], false}
  end

  def load_cluster_rooms do
    %{results: rooms, warnings: warnings} = Cluster.list_rooms(%{})
    {rooms, warnings}
  rescue
    e ->
      Logger.warning("Failed to load cluster rooms: #{inspect(e)}")
      {[], ["cluster query failed"]}
  end

  def safe_latest_update do
    Council.latest_room_update()
  rescue
    _e in [DBConnection.ConnectionError, Exqlite.Error] -> nil
  end

  # -- Safe aggregate wrappers --

  # Every safe_* wrapper below shares one shape: skip the DB call when
  # disconnected (returning a default value the caller can render against), and
  # degrade to that same default — logging why — if the shared SQLite
  # connection drops mid-call instead of crashing the LiveView.
  defp safe_call(default, label, fun) do
    fun.()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load #{label}: #{inspect(e)}")
      default
  end

  def safe_room_counts(false), do: %{}

  def safe_room_counts(true),
    do: safe_call(%{}, "room counts", &Council.all_room_message_counts/0)

  def safe_participant_counts(false), do: %{}

  def safe_participant_counts(true),
    do: safe_call(%{}, "participant counts", &Council.all_room_participant_counts/0)

  def safe_latest_ids(false), do: %{}

  def safe_latest_ids(true),
    do: safe_call(%{}, "latest message ids", &Council.all_room_latest_message_ids/0)

  def safe_synthesis_flags(false), do: MapSet.new()

  def safe_synthesis_flags(true),
    do: safe_call(MapSet.new(), "synthesis flags", &Council.all_room_synthesis_flags/0)

  def safe_type_counts(false), do: %{}

  def safe_type_counts(true),
    do: safe_call(%{}, "type counts", &Council.all_room_full_type_counts/0)

  def safe_time_ranges(false), do: %{}

  def safe_time_ranges(true),
    do: safe_call(%{}, "time ranges", &Council.all_room_time_ranges/0)

  def safe_room_participants(_room_id, false), do: []

  def safe_room_participants(room_id, true),
    do:
      safe_call([], "room participants", fn -> Council.room_participants_with_counts(room_id) end)

  # -- Pure helpers --

  def last_message_id([]), do: ""
  # Messages are ordered pinned-first for display, so List.last is not the newest
  # id. The poll cursor must be the true max id (UUIDv7 is lexicographically
  # time-ordered); otherwise a pinned newest message wedges the poll into
  # re-querying the same row every tick.
  def last_message_id(messages), do: messages |> Enum.map(& &1.id) |> Enum.max()

  def group_rooms_by_project(rooms) do
    rooms
    |> Enum.group_by(&room_project/1)
    |> Enum.sort_by(fn {project, _} -> if project == "ungrouped", do: "zzz", else: project end)
  end

  defp room_project(%{project: project}) when project in [nil, ""], do: "ungrouped"
  defp room_project(%{project: project}), do: project

  def filter_rooms(rooms, ""), do: rooms

  def filter_rooms(rooms, query) do
    q = String.downcase(query)

    Enum.filter(rooms, fn room ->
      room.id
      |> Kernel.<>(" ")
      |> Kernel.<>(room.description || "")
      |> String.downcase()
      |> String.contains?(q)
    end)
  end
end
