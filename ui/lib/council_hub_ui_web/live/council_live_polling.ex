defmodule CouncilHubUiWeb.CouncilLivePolling do
  @moduledoc """
  Pure polling and loading helpers extracted from CouncilLive.
  All functions accept and return Phoenix.LiveView sockets or plain values.
  """

  import Phoenix.Component, only: [assign: 2]
  import Phoenix.LiveView, only: [stream: 4]
  alias CouncilHubUi.{Council, Cluster}
  require Logger

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
          |> assign(last_msg_id: new_last_id, has_messages: true)
        end
    end
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

  def safe_room_counts(false), do: %{}

  def safe_room_counts(true) do
    Council.all_room_message_counts()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load room counts: #{inspect(e)}")
      %{}
  end

  def safe_participant_counts(false), do: %{}

  def safe_participant_counts(true) do
    Council.all_room_participant_counts()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load participant counts: #{inspect(e)}")
      %{}
  end

  def safe_latest_ids(false), do: %{}

  def safe_latest_ids(true) do
    Council.all_room_latest_message_ids()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load latest message ids: #{inspect(e)}")
      %{}
  end

  def safe_synthesis_flags(false), do: MapSet.new()

  def safe_synthesis_flags(true) do
    Council.all_room_synthesis_flags()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load synthesis flags: #{inspect(e)}")
      MapSet.new()
  end

  def safe_type_counts(false), do: %{}

  def safe_type_counts(true) do
    Council.all_room_full_type_counts()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load type counts: #{inspect(e)}")
      %{}
  end

  def safe_time_ranges(false), do: %{}

  def safe_time_ranges(true) do
    Council.all_room_time_ranges()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load time ranges: #{inspect(e)}")
      %{}
  end

  def safe_room_participants(_room_id, false), do: []

  def safe_room_participants(room_id, true) do
    Council.room_participants_with_counts(room_id)
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load room participants: #{inspect(e)}")
      []
  end

  # -- Pure helpers --

  def last_message_id([]), do: ""
  def last_message_id(messages), do: List.last(messages).id

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
