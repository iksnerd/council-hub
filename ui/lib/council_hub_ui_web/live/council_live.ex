defmodule CouncilHubUiWeb.CouncilLive do
  use CouncilHubUiWeb, :live_view

  alias CouncilHubUi.Council
  import CouncilHubUiWeb.CouncilComponents

  require Logger

  @poll_interval 1_000
  @rooms_poll_interval 5_000

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket) do
      schedule_poll(:poll_messages, @poll_interval)
      schedule_poll(:poll_rooms, @rooms_poll_interval)
    end

    {rooms, db_connected} = load_rooms()

    {:ok,
     socket
     |> assign(
       rooms: rooms,
       rooms_by_project: group_rooms_by_project(rooms),
       room_counts: safe_room_counts(db_connected),
       active_room: nil,
       last_msg_id: 0,
       collapsed_summaries: MapSet.new(),
       show_system_prompt: false,
       page_title: "Council Hub",
       db_connected: db_connected,
       room_filter: "",
       last_room_update: nil,
       has_messages: false
     )
     |> stream(:messages, [])}
  end

  @impl true
  def handle_params(%{"room_id" => room_id}, _uri, socket) do
    case Council.get_room(room_id) do
      nil ->
        {:noreply,
         socket
         |> put_flash(:error, "Room '#{room_id}' not found")
         |> push_navigate(to: ~p"/")}

      room ->
        messages = Council.list_messages_for_room(room_id)
        last_id = last_message_id(messages)

        {:noreply,
         socket
         |> assign(
           active_room: room,
           last_msg_id: last_id,
           show_system_prompt: false,
           page_title: "Council Hub · #{room.id}",
           has_messages: messages != []
         )
         |> stream(:messages, messages, reset: true)}
    end
  end

  def handle_params(_params, _uri, socket) do
    {:noreply,
     socket
     |> assign(active_room: nil, last_msg_id: 0)
     |> stream(:messages, [], reset: true)}
  end

  # -- Polling callbacks --

  @impl true
  def handle_info(:poll_messages, socket) do
    schedule_poll(:poll_messages, @poll_interval)

    socket
    |> poll_active_room_messages()
    |> then(&{:noreply, &1})
  end

  def handle_info(:poll_rooms, socket) do
    schedule_poll(:poll_rooms, @rooms_poll_interval)

    # Fast check: skip full reload if nothing changed
    latest = safe_latest_update()

    if latest == socket.assigns.last_room_update do
      {:noreply, socket}
    else
      poll_rooms_full(socket, latest)
    end
  end

  defp poll_rooms_full(socket, latest) do
    {rooms, db_connected} = load_rooms()
    new_counts = safe_room_counts(db_connected)

    socket =
      assign(socket,
        rooms: rooms,
        rooms_by_project: group_rooms_by_project(rooms),
        room_counts: new_counts,
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

  # -- Events --

  @impl true
  def handle_event("toggle_summary", %{"id" => id_str}, socket) do
    id = String.to_integer(id_str)
    collapsed = socket.assigns.collapsed_summaries

    collapsed =
      if MapSet.member?(collapsed, id),
        do: MapSet.delete(collapsed, id),
        else: MapSet.put(collapsed, id)

    {:noreply, assign(socket, collapsed_summaries: collapsed)}
  end

  def handle_event("toggle_system_prompt", _params, socket) do
    {:noreply, assign(socket, show_system_prompt: !socket.assigns.show_system_prompt)}
  end

  def handle_event("filter_rooms", %{"query" => query}, socket) do
    {:noreply, assign(socket, room_filter: query)}
  end

  # -- Helpers --

  defp schedule_poll(msg, interval), do: Process.send_after(self(), msg, interval)

  defp poll_active_room_messages(%{assigns: %{active_room: nil}} = socket), do: socket

  defp poll_active_room_messages(%{assigns: %{active_room: room, last_msg_id: last_id}} = socket) do
    case Council.get_messages_since(room.id, last_id) do
      [] ->
        socket

      new_messages ->
        socket
        |> stream(:messages, new_messages, at: -1)
        |> assign(last_msg_id: last_message_id(new_messages), has_messages: true)
    end
  end

  defp safe_latest_update do
    Council.latest_room_update()
  rescue
    _e in [DBConnection.ConnectionError, Exqlite.Error] -> nil
  end

  defp last_message_id([]), do: 0
  defp last_message_id(messages), do: List.last(messages).id

  defp load_rooms do
    {Council.list_rooms(), true}
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load rooms: #{inspect(e)}")
      {[], false}
  end

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

  defp safe_room_counts(false), do: %{}

  defp safe_room_counts(true) do
    Council.all_room_message_counts()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load room counts: #{inspect(e)}")
      %{}
  end
end
