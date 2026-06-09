defmodule CouncilHubUiWeb.RoomComponents do
  @moduledoc "Room card and room header function components."

  use Phoenix.Component
  import Phoenix.HTML, only: [raw: 1]
  import CouncilHubUiWeb.CouncilHelpers

  # -- Room Card (sidebar) --

  attr :room, :map, required: true
  attr :active, :boolean, default: false
  attr :count, :integer, default: 0
  attr :participants, :integer, default: 0
  attr :source_node, :string, default: nil
  attr :latest_id, :string, default: nil
  attr :compiled, :boolean, default: false
  attr :type_counts, :map, default: %{}
  attr :time_range, :any, default: nil

  def room_card(assigns) do
    ~H"""
    <.link
      patch={"/rooms/#{@room.id}"}
      class={[
        "block px-2 py-2 rounded transition-all duration-100 group border",
        if(@active,
          do: "bg-[var(--ch-active-bg)] border-[var(--ch-active-border)]",
          else: "border-transparent hover:bg-[var(--ch-hover-bg)] hover:border-[var(--ch-border)]"
        ),
        if(room_health_flags(@room).stale,
          do: "border-l-2 border-l-red-500/50",
          else:
            if(room_health_flags(@room).needs_synthesis,
              do: "border-l-2 border-l-amber-500/50",
              else: nil
            )
        )
      ]}
    >
      <%!-- Row 1: room ID + indicators --%>
      <div class="flex items-center justify-between gap-1.5">
        <span class="text-[12px] font-medium text-[var(--ch-text-hi)] truncate leading-tight">
          {@room.id}
        </span>
        <div class="flex items-center gap-1.5 shrink-0">
          <span :if={@count > 0} class="text-[10px] text-[var(--ch-text-xs)] font-mono">
            {@count}
          </span>
          <span
            :if={@compiled}
            class="text-[9px] text-purple-300/80 font-medium"
            title="Has synthesis"
          >
            S
          </span>
          <span
            class={["w-1.5 h-1.5 rounded-full shrink-0", status_dot_class(@room.status)]}
            title={@room.status}
          >
          </span>
        </div>
      </div>
      <%!-- Row 2: description --%>
      <p
        :if={present?(@room.description)}
        class="text-[11px] text-[var(--ch-text-lo)] leading-snug mt-0.5 truncate"
      >
        {@room.description}
      </p>
      <%!-- Row 3: metadata chips --%>
      <div class="flex items-center gap-2 mt-1 flex-wrap">
        <div
          :if={@room.updated_at}
          id={"room-time-#{@room.id}"}
          phx-hook="RelativeTime"
          data-timestamp={NaiveDateTime.to_iso8601(@room.updated_at)}
          class="text-[10px] text-[var(--ch-text-xs)] font-mono"
        >
        </div>
        <%!-- Participant count --%>
        <span
          :if={@participants > 0}
          class="text-[10px] text-[var(--ch-text-xs)] font-mono"
          title="Participants"
        >
          {@participants}p
        </span>
        <%!-- Full type breakdown --%>
        <span
          :if={format_type_counts(@type_counts)}
          class="text-[10px] text-[var(--ch-text-lo)] font-mono"
          title="Message type breakdown"
        >
          {format_type_counts(@type_counts)}
        </span>
        <%!-- Tech stack --%>
        <span
          :if={present?(Map.get(@room, :tech_stack))}
          class="text-[9px] text-purple-300/70 font-mono uppercase tracking-wide"
          title="Tech stack"
        >
          {Map.get(@room, :tech_stack)}
        </span>
        <%!-- Tags (up to 3, skip noise tags) --%>
        <%= for tag <- parse_tags(Map.get(@room, :tags)) |> Enum.reject(&(&1 in ~w(stale needs-synthesis))) |> Enum.take(3) do %>
          <span class="text-[9px] text-[var(--ch-text-xs)] font-mono bg-[var(--ch-raised)] px-1 rounded border border-[var(--ch-border)]">
            {tag}
          </span>
        <% end %>
        <%!-- Source node --%>
        <span
          :if={present?(@source_node)}
          class="text-[10px] text-[var(--ch-text-xs)] font-mono"
        >
          {short_node(@source_node)}
        </span>
      </div>
    </.link>
    """
  end

  # -- Room Header --

  attr :room, :map, required: true
  attr :count, :integer, default: 0
  attr :show_system_prompt, :boolean, default: false
  attr :editing_tags, :boolean, default: false
  attr :tag_input, :string, default: ""
  attr :participants, :list, default: []
  attr :time_range, :any, default: nil
  attr :type_counts, :map, default: %{}

  def room_header(assigns) do
    assigns = assign(assigns, :repo_link, repo_url(Map.get(assigns.room, :repo, "")))

    ~H"""
    <header class="bg-[var(--ch-surface)]/90 border-b border-[var(--ch-border)] px-4 py-3 shrink-0 backdrop-blur-sm">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <%!-- Title row --%>
          <div class="flex items-center gap-2 mb-1">
            <h2 class="text-[15px] font-semibold text-[var(--ch-text-hi)] truncate">{@room.id}</h2>
            <span
              class={["w-2 h-2 rounded-full shrink-0", status_dot_class(@room.status)]}
              title={@room.status}
            >
            </span>
            <span class="text-[10px] text-[var(--ch-text-lo)] uppercase tracking-wider">
              {@room.status}
            </span>
          </div>
          <%!-- Description --%>
          <p
            :if={present?(@room.description)}
            class="text-[12px] text-[var(--ch-text-mid)] leading-snug"
          >
            {@room.description}
          </p>
          <%!-- Participant badges --%>
          <div :if={@participants != []} class="flex items-center gap-1.5 mt-1.5 flex-wrap">
            <span class="text-[10px] text-[var(--ch-text-xs)] font-mono uppercase tracking-wider shrink-0">
              participants:
            </span>
            <span
              :for={{author, msg_count} <- @participants}
              class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono border border-[var(--ch-border)]"
              style={"color: #{author_hex(author)}; background: #{author_hex(author)}12"}
              title={"#{author}: #{msg_count} messages"}
            >
              {author}
              <span class="text-[var(--ch-text-xs)] text-[9px]">{msg_count}</span>
            </span>
          </div>
        </div>
        <%!-- Right meta column --%>
        <div class="flex flex-col items-end gap-1 shrink-0 text-[10px] text-[var(--ch-text-lo)] font-mono">
          <span>{@count} msgs</span>
          <span
            :if={format_type_counts(@type_counts)}
            class="text-[var(--ch-text-xs)]"
            title="Type breakdown"
          >
            {format_type_counts(@type_counts)}
          </span>
          <span :if={present?(Map.get(@room, :source_node))} class="text-[var(--ch-text-xs)]">
            {Map.get(@room, :source_node)}
          </span>
          <a
            :if={@repo_link != ""}
            href={@repo_link}
            target="_blank"
            rel="noopener noreferrer"
            class="text-[var(--ch-text-xs)] hover:text-[var(--ch-text-mid)] transition-colors font-mono"
            title="Repository for {sha:...} commit links"
          >
            {Map.get(@room, :repo, "")}
          </a>
          <span :if={@room.created_at}>{format_date(@room.created_at)}</span>
          <span :if={@time_range} class="text-[var(--ch-text-xs)]" title="First → last message">
            {format_time_range(elem(@time_range, 0), elem(@time_range, 1))}
          </span>
          <div
            :if={Map.get(@room, :updated_at)}
            id={"header-updated-#{@room.id}"}
            phx-hook="RelativeTime"
            data-timestamp={NaiveDateTime.to_iso8601(Map.get(@room, :updated_at))}
            class="text-[var(--ch-text-xs)]"
          >
          </div>
        </div>
      </div>

      <%!-- Metadata chips --%>
      <div class="flex items-center gap-1.5 mt-2 flex-wrap">
        <span
          :if={present?(@room.project)}
          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-[var(--ch-active-bg)] text-[var(--ch-text-hi)] text-[10px] border border-[var(--ch-active-border)]"
        >
          {String.upcase(@room.project)}
        </span>
        <span
          :if={present?(@room.tech_stack)}
          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-purple-500/10 text-purple-300 text-[10px] border border-purple-500/20"
        >
          {String.upcase(@room.tech_stack)}
        </span>
        <span
          :for={tag <- parse_tags(@room.tags)}
          class="px-1.5 py-0.5 rounded bg-[var(--ch-raised)] text-[var(--ch-text-lo)] text-[10px] border border-[var(--ch-border)]"
        >
          {tag}
        </span>

        <.link
          :for={related <- parse_tags(Map.get(@room, :related_rooms, ""))}
          patch={"/rooms/#{related}"}
          class="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-emerald-500/8 text-emerald-300/80 text-[10px] border border-emerald-500/20 hover:bg-emerald-500/15 transition-colors cursor-pointer"
        >
          <span class="hero-link w-2.5 h-2.5 opacity-60"></span>
          {related}
        </.link>

        <div class="flex items-center gap-1 ml-auto flex-wrap justify-end">
          <%!-- Tag editor --%>
          <div :if={@editing_tags} class="flex items-center gap-1">
            <form phx-submit="save_tags" class="flex items-center gap-1">
              <input
                type="text"
                name="tags"
                value={@tag_input}
                placeholder="tag1,tag2"
                autofocus
                class="px-1.5 py-0.5 rounded bg-[var(--ch-raised)] border border-[var(--ch-border-mid)] text-[var(--ch-text-hi)] text-[10px] font-mono w-40 focus:outline-none focus:border-[var(--ch-focus-border)]"
              />
              <button
                type="submit"
                class="px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-300 text-[10px] border border-emerald-500/20 hover:bg-emerald-500/20 transition-colors cursor-pointer"
              >
                save
              </button>
              <button
                type="button"
                phx-click="cancel_edit_tags"
                class="px-1.5 py-0.5 rounded bg-[var(--ch-raised)] text-[var(--ch-text-lo)] text-[10px] hover:bg-[var(--ch-active-bg)] transition-colors cursor-pointer"
              >
                cancel
              </button>
            </form>
          </div>
          <button
            :if={not @editing_tags}
            phx-click="edit_tags"
            title="Edit tags"
            class="px-1.5 py-0.5 rounded text-[var(--ch-text-lo)] text-[10px] hover:text-[var(--ch-text-mid)] hover:bg-[var(--ch-hover-bg)] transition-colors cursor-pointer"
          >
            tags
          </button>
          <button
            phx-click="check_room_health"
            phx-value-room-id={@room.id}
            title="Run lint"
            class="px-1.5 py-0.5 rounded text-[var(--ch-text-lo)] text-[10px] hover:text-[var(--ch-text-mid)] hover:bg-[var(--ch-hover-bg)] transition-colors cursor-pointer"
          >
            lint
          </button>
          <button
            :if={@room.status == "resolved"}
            phx-click="archive_room"
            phx-value-room-id={@room.id}
            title="Archive"
            class="px-1.5 py-0.5 rounded text-red-400/80 text-[10px] hover:text-red-300 hover:bg-red-500/10 transition-colors cursor-pointer"
          >
            archive
          </button>
          <a
            href={"/rooms/#{@room.id}/export"}
            download={"#{@room.id}.md"}
            class="px-1.5 py-0.5 rounded text-[var(--ch-text-lo)] text-[10px] hover:text-[var(--ch-text-mid)] hover:bg-[var(--ch-hover-bg)] transition-colors"
          >
            export
          </a>
          <button
            :if={present?(@room.system_prompt)}
            phx-click="toggle_system_prompt"
            aria-label={if @show_system_prompt, do: "Hide system prompt", else: "Show system prompt"}
            aria-expanded={to_string(@show_system_prompt)}
            class="px-1.5 py-0.5 rounded text-[var(--ch-text-lo)] text-[10px] hover:text-[var(--ch-text-mid)] hover:bg-[var(--ch-hover-bg)] transition-colors cursor-pointer"
          >
            {if @show_system_prompt, do: "- prompt", else: "+ prompt"}
          </button>
        </div>
      </div>

      <%!-- System prompt content --%>
      <div
        :if={@show_system_prompt && present?(@room.system_prompt)}
        class="mt-2 p-3 rounded bg-[var(--ch-raised)] border border-[var(--ch-border-mid)] text-[12px] text-[var(--ch-text-mid)] council-prose"
      >
        {raw(render_markdown(@room.system_prompt))}
      </div>
    </header>
    """
  end
end
