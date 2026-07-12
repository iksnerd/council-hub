defmodule CouncilHubUiWeb.NotebookLive do
  @moduledoc """
  Project notebook: a read-only chronological timeline compiled from typed
  messages across every room in a project, grouped by day. The UI twin of the
  `read_notebook` MCP tool — same query (CouncilHubUi.CouncilNotebook), same
  defaults (decision/action/synthesis/note), same per-room {sha:...} resolution.
  """
  use CouncilHubUiWeb, :live_view

  require Logger

  alias CouncilHubUi.Council
  alias CouncilHubUi.McpClient

  import CouncilHubUiWeb.MessageComponents, only: [retracted_badge: 1, retracted_body: 1]

  import CouncilHubUiWeb.CouncilHelpers,
    only: [
      type_color: 1,
      author_hex: 1,
      format_timestamp: 1,
      format_date: 1,
      render_markdown: 1,
      resolve_commit_refs: 2,
      status_dot_class: 1,
      status_badge_class: 1,
      truncate: 2
    ]

  @refresh_interval 5_000
  @all_types ~w(decision plan action synthesis note critique review draft thought message)
  @default_types ~w(decision plan action synthesis note)

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket), do: Process.send_after(self(), :refresh, @refresh_interval)

    {:ok,
     assign(socket,
       page_title: "Notebook",
       projects: Council.list_projects(),
       all_types: @all_types,
       view_compact: false,
       show_done: false,
       compose_author: "",
       outline_inline: [],
       outline_doing_tasks: [],
       outline_open_tasks: [],
       outline_done_tasks: [],
       outline_in_flight: [],
       outline_done: []
     )}
  end

  @impl true
  def handle_params(params, _uri, socket) do
    project = Map.get(params, "project", "") |> default_project(socket.assigns.projects)
    types = parse_types(Map.get(params, "types", ""))
    notebook_id = Map.get(params, "notebook", "")

    {:noreply,
     socket
     |> assign(
       project: project,
       selected_types: types,
       notebook_id: notebook_id,
       view_compact: Map.get(params, "compact", "") == "1"
     )
     |> load_entries()}
  end

  @impl true
  def handle_event("select_project", %{"project" => project}, socket) do
    {:noreply, patch_to(socket, project, socket.assigns.selected_types)}
  end

  @impl true
  def handle_event("toggle_done", _params, socket) do
    {:noreply, assign(socket, show_done: !socket.assigns.show_done)}
  end

  @impl true
  def handle_event("toggle_type", %{"type" => type}, socket) do
    selected = socket.assigns.selected_types

    types =
      if type in selected,
        do: Enum.reject(selected, &(&1 == type)),
        else: selected ++ [type]

    # An empty selection would fall back to the defaults server-side, which
    # reads as "my last toggle un-toggled everything else" — keep the last one.
    types = if types == [], do: [type], else: types

    {:noreply, patch_to(socket, socket.assigns.project, types)}
  end

  # Human note composer: the note is born in the dialog ledger (a typed
  # message in a project room, posted through the Go server's localhost-only
  # UI endpoint) — never written into the notebook tables directly. Outlines
  # then transclude it. See the Engelbart design thought in notebook-feature.
  @impl true
  def handle_event(
        "post_note",
        %{"room_id" => room_id, "author" => author, "type" => type, "message" => message},
        socket
      ) do
    message = String.trim(message)
    author = String.trim(author)

    cond do
      message == "" or author == "" ->
        {:noreply, put_flash(socket, :error, "Name and note are both required")}

      room_id == "" ->
        {:noreply, put_flash(socket, :error, "Pick a room — notes live in the dialog ledger")}

      true ->
        case McpClient.post_to_room(room_id, author, message, type) do
          :ok ->
            {:noreply,
             socket
             |> assign(compose_author: author)
             |> put_flash(:info, "Posted to #{room_id} as #{type}")
             |> load_entries()}

          {:error, reason} ->
            Logger.warning("notebook post_note failed: #{inspect(reason)}")

            {:noreply,
             put_flash(socket, :error, "Post failed — is the MCP server running in HTTP mode?")}
        end
    end
  end

  @impl true
  def handle_event("pin_entry", %{"message-id" => message_id, "notebook" => notebook_id}, socket) do
    case McpClient.add_notebook_entry(notebook_id, %{ref_id: message_id}) do
      :ok ->
        {:noreply,
         socket
         |> put_flash(:info, "Pinned ##{String.slice(message_id, 0, 8)} into #{notebook_id}")
         |> load_entries()}

      {:error, reason} ->
        Logger.warning("notebook pin_entry failed: #{inspect(reason)}")

        {:noreply,
         put_flash(socket, :error, "Pin failed — is the MCP server running in HTTP mode?")}
    end
  end

  @impl true
  def handle_info(:refresh, socket) do
    Process.send_after(self(), :refresh, @refresh_interval)

    {:noreply,
     socket
     |> assign(projects: Council.list_projects())
     |> load_entries()}
  end

  ## Helpers

  defp patch_to(socket, project, types) do
    push_patch(socket,
      to: ~p"/notebook?#{notebook_view_params(project, types, socket.assigns[:view_compact])}"
    )
  end

  @doc "Builds the /notebook query params, carrying the compact ViewSpec flag through the URL."
  def notebook_view_params(project, types, compact) do
    base = %{project: project, types: Enum.join(types, ",")}
    if compact, do: Map.put(base, :compact, "1"), else: base
  end

  # Outline mode: ?notebook=<id> shows a curated outline instead of the
  # compiled timeline. The notebook's project wins over the project param so
  # deep links stay consistent.
  defp load_outline(socket, notebook_id) do
    case Council.get_notebook(notebook_id) do
      nil ->
        socket
        |> assign(notebook: nil, outline: [], entries: [], entry_count: 0, days: [], rooms: [])
        |> put_flash(:error, "Notebook '#{notebook_id}' not found")

      notebook ->
        outline = Council.outline_entries(notebook_id)

        {inline, doing_tasks, open_tasks, done_tasks, in_flight, done} =
          partition_outline(outline)

        assign(socket,
          notebook: notebook,
          outline: outline,
          outline_inline: inline,
          outline_doing_tasks: doing_tasks,
          outline_open_tasks: open_tasks,
          outline_done_tasks: done_tasks,
          outline_in_flight: in_flight,
          outline_done: done,
          project: notebook.project,
          entries: [],
          entry_count: length(outline),
          days: []
        )
    end
  end

  # Splits a curated outline into its document spine (prose + message refs, in
  # authored order) and its self-sorting work lists: tasks regrouped by their own
  # done state (☐ Open / ☑ Done), and room_refs regrouped by transcluded live
  # status (🔄 In flight / ✅ Done). Both re-partition without hand-editing — a task
  # is checked, a room is resolved, and the item moves.
  defp partition_outline(outline) do
    {tasks, rest} = Enum.split_with(outline, &(&1.kind == "task"))
    {room_refs, inline} = Enum.split_with(rest, &(&1.kind == "room_ref"))
    doing_tasks = Enum.filter(tasks, &(&1.status == "doing"))
    done_tasks = Enum.filter(tasks, &(&1.status == "done"))
    open_tasks = Enum.filter(tasks, &(&1.status not in ["doing", "done"]))
    {done, in_flight} = Enum.split_with(room_refs, &room_ref_done?/1)
    {inline, doing_tasks, open_tasks, done_tasks, in_flight, done}
  end

  defp room_ref_done?(%{ref_found: true, room_status: status}),
    do: status in ["resolved", "archived"]

  defp room_ref_done?(_), do: false

  # One work-list item in a curated notebook: a room transcluded by its live
  # status, topic, and latest decision/action. A dangling ref (room deleted or
  # node-local elsewhere) renders as a warning in place.
  attr :entry, :map, required: true

  def room_ref_card(assigns) do
    ~H"""
    <%= if @entry.ref_found do %>
      <div
        id={"outline-#{@entry.id}"}
        class="bg-[var(--ch-surface)] border border-[var(--ch-border)] rounded-lg p-3"
      >
        <div class="flex items-center gap-2 flex-wrap">
          <span class={["w-1.5 h-1.5 rounded-full shrink-0", status_dot_class(@entry.room_status)]}></span>
          <.link
            navigate={~p"/rooms/#{@entry.room_id}"}
            class="text-[12px] font-mono text-[var(--ch-text-hi)] hover:underline"
          >
            {@entry.room_id}
          </.link>
          <span class={[
            "px-1.5 py-px rounded text-[10px] font-mono",
            status_badge_class(@entry.room_status)
          ]}>
            {@entry.room_status}
          </span>
          <span class="text-[11px] text-[var(--ch-text-lo)] truncate">
            {@entry.room_topic}
          </span>
        </div>
        <%= if @entry.content != "" do %>
          <div class="mt-1.5 text-[11px] text-[var(--ch-text-xs)] line-clamp-2">
            latest: {truncate(@entry.content, 200)}
          </div>
        <% end %>
      </div>
    <% else %>
      <div
        id={"outline-#{@entry.id}"}
        class="bg-amber-500/5 border border-amber-500/20 rounded-lg p-3 text-[11px] text-amber-200/90 font-mono"
      >
        ⚠ referenced room {String.slice(@entry.ref_id, 0, 12)} not found — deleted, or it lives on another cluster node
      </div>
    <% end %>
    """
  end

  # One checklist item in a curated notebook: a first-class task with its own
  # status (open / doing / done). Read-only here — tasks are driven from the MCP
  # tool (edit_notebook start/check/uncheck); the browser view just reflects state.
  attr :entry, :map, required: true

  def task_card(assigns) do
    ~H"""
    <div
      id={"outline-#{@entry.id}"}
      class="flex items-start gap-2 bg-[var(--ch-surface)] border border-[var(--ch-border)] rounded-lg px-3 py-2"
    >
      <span class="text-[13px] leading-5 shrink-0">{task_box(@entry.status)}</span>
      <span class={[
        "text-[12px] leading-5",
        if(@entry.status == "done",
          do: "text-[var(--ch-text-lo)] line-through",
          else: "text-[var(--ch-text-hi)]"
        )
      ]}>
        {@entry.prose}
      </span>
    </div>
    """
  end

  defp task_box("done"), do: "☑"
  defp task_box("doing"), do: "🔄"
  defp task_box(_), do: "☐"

  defp default_project("", [first | _]), do: first
  defp default_project(project, _projects), do: project

  # Builds the read-only relationship chips shown under a notebook entry: its
  # supersession (forward + backlink) and explicit typed links. Display-only —
  # the notebook is cross-room, so these don't scroll/navigate.
  def notebook_relations(entry) do
    supersedes =
      case Map.get(entry, :supersedes, "") do
        s when s in [nil, ""] -> []
        s -> [%{label: "→ supersedes ##{String.slice(s, 0, 8)}", warn: false}]
      end

    superseded =
      case Map.get(entry, :superseded_by, "") do
        s when s in [nil, ""] -> []
        s -> [%{label: "⚠ superseded by ##{String.slice(s, 0, 8)}", warn: true}]
      end

    links =
      for e <- Map.get(entry, :links, []) do
        arrow = if e.direction == :out, do: "→", else: "←"
        %{label: "#{arrow} #{e.relation} ##{String.slice(e.other_id, 0, 8)}", warn: false}
      end

    supersedes ++ superseded ++ links
  end

  defp parse_types(""), do: @default_types

  defp parse_types(csv) do
    case csv |> String.split(",", trim: true) |> Enum.filter(&(&1 in @all_types)) do
      [] -> @default_types
      types -> types
    end
  end

  defp load_entries(%{assigns: %{notebook_id: notebook_id}} = socket)
       when notebook_id != "" do
    socket
    |> load_outline(notebook_id)
    |> assign(notebooks: Council.list_notebooks(socket.assigns.project))
  end

  defp load_entries(socket) do
    %{project: project, selected_types: types} = socket.assigns

    entries =
      if project == "" do
        []
      else
        Council.notebook_entries(%{
          "project" => project,
          "types" => Enum.join(types, ",")
        })
      end

    rooms =
      if project == "",
        do: [],
        else: Council.list_rooms_filtered(%{"project" => project})

    assign(socket,
      notebook: nil,
      notebooks: Council.list_notebooks(project),
      rooms: rooms,
      outline: [],
      entries: entries,
      entry_count: length(entries),
      days: Enum.chunk_by(entries, &NaiveDateTime.to_date(&1.timestamp))
    )
  end
end
