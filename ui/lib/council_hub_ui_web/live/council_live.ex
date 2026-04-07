defmodule CouncilHubUiWeb.CouncilLive do
  use CouncilHubUiWeb, :live_view

  alias CouncilHubUi.{Council, Cluster}
  import CouncilHubUiWeb.CouncilComponents
  import CouncilHubUiWeb.CouncilHelpers, only: [short_node: 1]

  require Logger

  @poll_interval 3_000
  @rooms_poll_interval 5_000
  @cluster_poll_interval 3_000

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket) do
      schedule_poll(:poll_messages, @poll_interval)
      schedule_poll(:poll_rooms, @rooms_poll_interval)
      schedule_poll(:poll_cluster, @cluster_poll_interval)
    end

    {rooms, db_connected} = load_rooms()

    {:ok,
     socket
     |> assign(
       rooms: rooms,
       rooms_by_project: group_rooms_by_project(rooms),
       room_counts: safe_room_counts(db_connected),
       participant_counts: safe_participant_counts(db_connected),
       latest_ids: safe_latest_ids(db_connected),
       synthesis_flags: safe_synthesis_flags(db_connected),
       active_room: nil,
       last_msg_id: "",
       collapsed_summaries: MapSet.new(),
       show_system_prompt: false,
       page_title: "Council Hub",
       db_connected: db_connected,
       room_filter: "",
       last_room_update: nil,
       has_messages: false,
       type_filter: "all",
       message_search: "",
       searching: false,
       show_search_filters: false,
       search_author: "",
       search_since: "",
       search_until: "",
       nodes: [Node.self() | Node.list()],
       cluster_wide: false,
       cluster_warnings: []
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
           has_messages: messages != [],
           type_filter: "all",
           message_search: "",
           searching: false,
           show_search_filters: false,
           search_author: "",
           search_since: "",
           search_until: ""
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

    if socket.assigns.cluster_wide do
      poll_rooms_full(socket, socket.assigns.last_room_update)
    else
      # Fast check: skip full reload if nothing changed
      latest = safe_latest_update()

      if latest == socket.assigns.last_room_update do
        {:noreply, socket}
      else
        poll_rooms_full(socket, latest)
      end
    end
  end

  def handle_info(:poll_cluster, socket) do
    schedule_poll(:poll_cluster, @cluster_poll_interval)
    {:noreply, assign(socket, nodes: [Node.self() | Node.list()])}
  end

  defp poll_rooms_full(socket, latest) do
    if socket.assigns.cluster_wide do
      {rooms, warnings} = load_cluster_rooms()

      {:noreply,
       assign(socket,
         rooms: rooms,
         rooms_by_project: group_rooms_by_project(rooms),
         cluster_warnings: warnings
       )}
    else
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
  end

  # -- Events --

  @impl true
  def handle_event("toggle_summary", %{"id" => id_str}, socket) do
    id = id_str
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

  def handle_event("toggle_cluster_wide", _params, socket) do
    new_val = !socket.assigns.cluster_wide

    {rooms, db_connected, warnings} =
      if new_val do
        {rooms, warns} = load_cluster_rooms()
        {rooms, socket.assigns.db_connected, warns}
      else
        {rooms, connected} = load_rooms()
        {rooms, connected, []}
      end

    {:noreply,
     assign(socket,
       cluster_wide: new_val,
       rooms: rooms,
       rooms_by_project: group_rooms_by_project(rooms),
       db_connected: db_connected,
       cluster_warnings: warnings
     )}
  end

  def handle_event("filter_rooms", %{"query" => query}, socket) do
    {:noreply, assign(socket, room_filter: query)}
  end

  def handle_event("filter_type", %{"type" => type}, socket) do
    case socket.assigns.active_room do
      nil ->
        {:noreply, assign(socket, type_filter: type)}

      room ->
        messages = Council.list_messages_for_room(room.id, type)
        last_id = last_message_id(messages)

        {:noreply,
         socket
         |> assign(
           type_filter: type,
           last_msg_id: last_id,
           has_messages: messages != [],
           searching: false,
           message_search: ""
         )
         |> stream(:messages, messages, reset: true)}
    end
  end

  def handle_event("search_messages", %{"query" => ""}, socket) do
    case socket.assigns.active_room do
      nil ->
        {:noreply, assign(socket, message_search: "", searching: false)}

      room ->
        messages = Council.list_messages_for_room(room.id, socket.assigns.type_filter)
        last_id = last_message_id(messages)

        {:noreply,
         socket
         |> assign(
           message_search: "",
           searching: false,
           last_msg_id: last_id,
           has_messages: messages != []
         )
         |> stream(:messages, messages, reset: true)}
    end
  end

  def handle_event("search_messages", %{"query" => query}, socket) do
    case socket.assigns.active_room do
      nil ->
        {:noreply, assign(socket, message_search: query)}

      room ->
        results = Council.search_messages_in_room(room.id, query, socket.assigns.type_filter)

        {:noreply,
         socket
         |> assign(message_search: query, searching: true, has_messages: results != [])
         |> stream(:messages, results, reset: true)}
    end
  end

  def handle_event("react", %{"message-id" => message_id, "emoji" => emoji}, socket) do
    author = socket.assigns[:current_user] || "dashboard"

    case CouncilHubUi.McpClient.react_to_message(message_id, emoji, author) do
      :ok -> :ok
      {:error, reason} -> Logger.warning("react_to_message failed: #{inspect(reason)}")
    end

    {:noreply, socket}
  end

  def handle_event("toggle_search_filters", _params, socket) do
    {:noreply, assign(socket, show_search_filters: !socket.assigns.show_search_filters)}
  end

  def handle_event("apply_search_filters", params, socket) do
    author = Map.get(params, "author", "")
    since = Map.get(params, "since", "")
    until_val = Map.get(params, "until", "")

    socket = assign(socket, search_author: author, search_since: since, search_until: until_val)

    case socket.assigns.active_room do
      nil ->
        {:noreply, socket}

      room ->
        has_advanced = author != "" or since != "" or until_val != ""

        {results, searching} =
          if has_advanced or socket.assigns.message_search != "" do
            msgs =
              Council.search_messages(%{
                "room_id" => room.id,
                "query" => socket.assigns.message_search,
                "author" => if(author == "", do: nil, else: author),
                "since" => if(since == "", do: nil, else: since),
                "until" => if(until_val == "", do: nil, else: until_val),
                "limit" => 200
              })

            {msgs, true}
          else
            msgs = Council.list_messages_for_room(room.id, socket.assigns.type_filter)
            {msgs, false}
          end

        last_id = last_message_id(results)

        {:noreply,
         socket
         |> assign(
           last_msg_id: last_id,
           searching: searching,
           has_messages: results != []
         )
         |> stream(:messages, results, reset: true)}
    end
  end

  def handle_event("toggle_status", %{"room-id" => room_id, "status" => current_status}, socket) do
    next_status =
      case current_status do
        "active" -> "paused"
        "paused" -> "resolved"
        _ -> "active"
      end

    case CouncilHubUi.McpClient.signal_status(room_id, next_status) do
      :ok -> :ok
      {:error, reason} -> Logger.warning("signal_status failed: #{inspect(reason)}")
    end

    {:noreply, socket}
  end

  # -- Helpers --

  defp schedule_poll(msg, interval), do: Process.send_after(self(), msg, interval)

  defp poll_active_room_messages(%{assigns: %{active_room: nil}} = socket), do: socket

  defp poll_active_room_messages(
         %{
           assigns: %{
             active_room: room,
             last_msg_id: last_id,
             type_filter: type_filter,
             searching: searching
           }
         } =
           socket
       ) do
    case Council.get_messages_since(room.id, last_id, type_filter) do
      [] ->
        socket

      new_messages ->
        new_last_id = last_message_id(new_messages)

        if searching do
          # Don't update the stream while searching — just track the new last_id
          assign(socket, last_msg_id: new_last_id)
        else
          socket
          |> stream(:messages, new_messages, at: -1)
          |> assign(last_msg_id: new_last_id, has_messages: true)
        end
    end
  end

  defp safe_latest_update do
    Council.latest_room_update()
  rescue
    _e in [DBConnection.ConnectionError, Exqlite.Error] -> nil
  end

  defp last_message_id([]), do: ""
  defp last_message_id(messages), do: List.last(messages).id

  defp load_rooms do
    {Council.list_rooms(), true}
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load rooms: #{inspect(e)}")
      {[], false}
  end

  defp load_cluster_rooms do
    %{results: rooms, warnings: warnings} = Cluster.list_rooms(%{})
    {rooms, warnings}
  rescue
    e ->
      Logger.warning("Failed to load cluster rooms: #{inspect(e)}")
      {[], ["cluster query failed"]}
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

  defp safe_participant_counts(false), do: %{}

  defp safe_participant_counts(true) do
    Council.all_room_participant_counts()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load participant counts: #{inspect(e)}")
      %{}
  end

  defp safe_latest_ids(false), do: %{}

  defp safe_latest_ids(true) do
    Council.all_room_latest_message_ids()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load latest message ids: #{inspect(e)}")
      %{}
  end

  defp safe_synthesis_flags(false), do: MapSet.new()

  defp safe_synthesis_flags(true) do
    Council.all_room_synthesis_flags()
  rescue
    e in [DBConnection.ConnectionError, Exqlite.Error] ->
      Logger.warning("Failed to load synthesis flags: #{inspect(e)}")
      MapSet.new()
  end
end
