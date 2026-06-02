defmodule CouncilHubUiWeb.CouncilLive do
  use CouncilHubUiWeb, :live_view

  alias CouncilHubUi.Council
  alias CouncilHubUiWeb.CouncilLivePolling, as: Polling
  import CouncilHubUiWeb.RoomComponents
  import CouncilHubUiWeb.MessageComponents
  import CouncilHubUiWeb.PanelComponents
  import CouncilHubUiWeb.CouncilHelpers, only: [short_node: 1]

  require Logger

  @poll_interval 3_000
  @rooms_poll_interval 5_000
  @cluster_poll_interval 3_000
  @mentions_poll_interval 10_000
  @archives_poll_interval 30_000

  @impl true
  def mount(_params, session, socket) do
    if connected?(socket) do
      Polling.schedule_poll(:poll_messages, @poll_interval)
      Polling.schedule_poll(:poll_rooms, @rooms_poll_interval)
      Polling.schedule_poll(:poll_cluster, @cluster_poll_interval)
      Polling.schedule_poll(:poll_mentions, @mentions_poll_interval)
      Polling.schedule_poll(:poll_archives, @archives_poll_interval)
    end

    {rooms, db_connected} = Polling.load_rooms()

    {:ok,
     socket
     |> assign(
       rooms: rooms,
       rooms_by_project: Polling.group_rooms_by_project(rooms),
       room_counts: Polling.safe_room_counts(db_connected),
       participant_counts: Polling.safe_participant_counts(db_connected),
       latest_ids: Polling.safe_latest_ids(db_connected),
       synthesis_flags: Polling.safe_synthesis_flags(db_connected),
       type_counts: Polling.safe_type_counts(db_connected),
       time_ranges: Polling.safe_time_ranges(db_connected),
       active_room: nil,
       active_room_participants: [],
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
       cluster_admin: session["cluster_admin"] == true,
       cluster_wide: false,
       cluster_warnings: [],
       editing_tags: false,
       tag_input: "",
       mentions: [],
       mention_author: System.get_env("COUNCIL_AUTHOR", "claude-code"),
       archives: [],
       active_archive: nil,
       compose_author: "human"
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
        last_id = Polling.last_message_id(messages)
        participants = Polling.safe_room_participants(room_id, socket.assigns.db_connected)

        {:noreply,
         socket
         |> assign(
           active_room: room,
           active_room_participants: participants,
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
    Polling.schedule_poll(:poll_messages, @poll_interval)

    socket
    |> Polling.poll_active_room_messages()
    |> then(&{:noreply, &1})
  end

  def handle_info(:poll_rooms, socket) do
    Polling.schedule_poll(:poll_rooms, @rooms_poll_interval)

    if socket.assigns.cluster_wide do
      Polling.poll_rooms_full(socket, socket.assigns.last_room_update)
    else
      latest = Polling.safe_latest_update()

      if latest == socket.assigns.last_room_update do
        {:noreply, socket}
      else
        Polling.poll_rooms_full(socket, latest)
      end
    end
  end

  def handle_info(:poll_cluster, socket) do
    Polling.schedule_poll(:poll_cluster, @cluster_poll_interval)
    {:noreply, assign(socket, nodes: [Node.self() | Node.list()])}
  end

  def handle_info(:poll_mentions, socket) do
    Polling.schedule_poll(:poll_mentions, @mentions_poll_interval)

    mentions =
      try do
        Council.get_mentions(socket.assigns.mention_author, 10)
      rescue
        _ -> socket.assigns.mentions
      end

    {:noreply, assign(socket, mentions: mentions)}
  end

  def handle_info(:poll_archives, socket) do
    Polling.schedule_poll(:poll_archives, @archives_poll_interval)

    archives =
      try do
        case CouncilHubUi.McpClient.list_archives() do
          {:ok, text} -> Jason.decode!(text)
          _ -> socket.assigns.archives
        end
      rescue
        _ -> socket.assigns.archives
      end

    {:noreply, assign(socket, archives: archives)}
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
        {rooms, warns} = Polling.load_cluster_rooms()
        {rooms, socket.assigns.db_connected, warns}
      else
        {rooms, connected} = Polling.load_rooms()
        {rooms, connected, []}
      end

    {:noreply,
     assign(socket,
       cluster_wide: new_val,
       rooms: rooms,
       rooms_by_project: Polling.group_rooms_by_project(rooms),
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
        last_id = Polling.last_message_id(messages)

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
        last_id = Polling.last_message_id(messages)

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

        last_id = Polling.last_message_id(results)

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

  def handle_event("archive_room", %{"room-id" => room_id}, socket) do
    case CouncilHubUi.McpClient.archive_room(room_id) do
      :ok ->
        {:noreply, put_flash(socket, :info, "Room '#{room_id}' archived.")}

      {:error, reason} ->
        Logger.warning("archive_room failed: #{inspect(reason)}")

        {:noreply,
         put_flash(socket, :error, "Archive failed — is the MCP server running in HTTP mode?")}
    end
  end

  def handle_event("check_room_health", %{"room-id" => room_id}, socket) do
    case CouncilHubUi.McpClient.check_room_health(room_id) do
      :ok ->
        {:noreply, put_flash(socket, :info, "Health check triggered for '#{room_id}'.")}

      {:error, reason} ->
        Logger.warning("check_room_health failed: #{inspect(reason)}")

        {:noreply,
         put_flash(
           socket,
           :error,
           "Linter check failed — is the MCP server running in HTTP mode?"
         )}
    end
  end

  def handle_event("edit_tags", _params, socket) do
    tag_input = Map.get(socket.assigns.active_room || %{}, :tags, "")
    {:noreply, assign(socket, editing_tags: true, tag_input: tag_input)}
  end

  def handle_event("cancel_edit_tags", _params, socket) do
    {:noreply, assign(socket, editing_tags: false, tag_input: "")}
  end

  def handle_event("view_archive", %{"room-id" => room_id}, socket) do
    content =
      case CouncilHubUi.McpClient.read_archive(room_id) do
        {:ok, text} -> text
        _ -> "Failed to load archive — is the MCP server running in HTTP mode?"
      end

    {:noreply, assign(socket, active_archive: %{room_id: room_id, content: content})}
  end

  def handle_event("close_archive", _params, socket) do
    {:noreply, assign(socket, active_archive: nil)}
  end

  def handle_event(
        "post_message",
        %{"author" => author, "type" => type, "message" => message},
        socket
      ) do
    message = String.trim(message)
    author = String.trim(author)

    cond do
      message == "" ->
        {:noreply, socket}

      is_nil(socket.assigns.active_room) ->
        {:noreply, socket}

      true ->
        room_id = socket.assigns.active_room.id

        case CouncilHubUi.McpClient.post_to_room(room_id, author, message, type) do
          :ok ->
            {:noreply,
             socket
             |> assign(compose_author: author)
             |> push_event("clear_form", %{id: "compose-form"})}

          {:error, reason} ->
            Logger.warning("post_to_room failed: #{inspect(reason)}")

            {:noreply,
             put_flash(socket, :error, "Post failed — is the MCP server running in HTTP mode?")}
        end
    end
  end

  def handle_event("save_tags", %{"tags" => tags}, socket) do
    room_id = socket.assigns.active_room.id

    normalized =
      tags
      |> String.split(",")
      |> Enum.map(&String.trim/1)
      |> Enum.reject(&(&1 == ""))
      |> Enum.join(",")

    case CouncilHubUi.McpClient.update_room_tags(room_id, normalized) do
      :ok ->
        {:noreply, assign(socket, editing_tags: false, tag_input: "")}

      {:error, reason} ->
        Logger.warning("update_room_tags failed: #{inspect(reason)}")

        {:noreply,
         socket
         |> assign(editing_tags: false, tag_input: "")
         |> put_flash(:error, "Tag update failed — is the MCP server running in HTTP mode?")}
    end
  end

  # -- Template helpers (public — called from council_live.html.heex) --

  def group_rooms_by_project(rooms), do: Polling.group_rooms_by_project(rooms)
  def filter_rooms(rooms, query), do: Polling.filter_rooms(rooms, query)
end
