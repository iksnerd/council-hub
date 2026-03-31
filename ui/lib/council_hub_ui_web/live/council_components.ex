defmodule CouncilHubUiWeb.CouncilComponents do
  @moduledoc """
  Function components for the Council Hub UI.
  """
  use Phoenix.Component
  import Phoenix.HTML, only: [raw: 1]
  import CouncilHubUiWeb.CouncilHelpers

  # -- Room Card (sidebar) --

  attr :room, :map, required: true
  attr :active, :boolean, default: false
  attr :count, :integer, default: 0

  def room_card(assigns) do
    ~H"""
    <.link
      patch={"/rooms/#{@room.id}"}
      class={[
        "block p-3 rounded-lg transition-all duration-150 group border",
        if(@active,
          do: "bg-amber-500/10 border-amber-500/30 shadow-sm shadow-amber-500/5",
          else: "border-transparent hover:bg-gray-800/50 hover:border-gray-700/40"
        )
      ]}
    >
      <div class="flex items-start justify-between gap-2 mb-1">
        <span class="text-[13px] font-semibold text-gray-200 truncate font-mono leading-tight">
          {@room.id}
        </span>
        <div class="flex items-center gap-1.5 shrink-0">
          <span :if={@count > 0} class="text-[10px] text-gray-500 font-mono">{@count}</span>
          <span class={[
            "inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium",
            status_badge_class(@room.status)
          ]}>
            <span class={"w-1.5 h-1.5 rounded-full #{status_dot_class(@room.status)}"}></span>
            {@room.status}
          </span>
        </div>
      </div>
      <p
        :if={present?(@room.description)}
        class="text-xs text-gray-400 leading-relaxed mb-1.5 line-clamp-2"
      >
        {@room.description}
      </p>
      <div :if={parse_tags(@room.tags) != []} class="flex items-center gap-1 flex-wrap">
        <span
          :for={tag <- Enum.take(parse_tags(@room.tags), 3)}
          class="inline-flex items-center px-1.5 py-0.5 rounded-md bg-gray-700/40 text-gray-500 text-[10px]"
        >
          {tag}
        </span>
      </div>
      <div
        :if={@room.updated_at}
        id={"room-time-#{@room.id}"}
        phx-hook="RelativeTime"
        data-timestamp={NaiveDateTime.to_iso8601(@room.updated_at)}
        class="mt-1.5 text-[10px] text-gray-600 font-mono"
      >
      </div>
    </.link>
    """
  end

  # -- Room Header --

  attr :room, :map, required: true
  attr :count, :integer, default: 0
  attr :show_system_prompt, :boolean, default: false

  def room_header(assigns) do
    ~H"""
    <header class="bg-gray-900/40 border-b border-gray-800/80 px-6 py-4 shrink-0 backdrop-blur-sm">
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-3 mb-1">
            <h2 class="text-lg font-bold text-gray-100 font-mono truncate">{@room.id}</h2>
            <span class={[
              "inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-xs font-medium shrink-0",
              status_badge_class(@room.status)
            ]}>
              <span class={"w-1.5 h-1.5 rounded-full #{status_dot_class(@room.status)}"}></span>
              {@room.status}
            </span>
          </div>
          <p
            :if={present?(@room.description)}
            class="text-sm text-gray-300 leading-relaxed"
          >
            {@room.description}
          </p>
        </div>
        <div class="flex flex-col items-end gap-1 shrink-0 text-right">
          <span class="text-xs text-gray-500 font-mono tabular-nums">{@count} msgs</span>
          <span :if={@room.created_at} class="text-[10px] text-gray-600 font-mono">
            {format_date(@room.created_at)}
          </span>
        </div>
      </div>

      <%!-- Metadata pills --%>
      <div class="flex items-center gap-2 mt-2.5 flex-wrap">
        <span
          :if={present?(@room.project)}
          class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md bg-blue-500/10 text-blue-400 text-xs border border-blue-500/15"
        >
          <span class="opacity-50">project</span> {@room.project}
        </span>
        <span
          :if={present?(@room.tech_stack)}
          class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md bg-purple-500/10 text-purple-400 text-xs border border-purple-500/15"
        >
          <span class="opacity-50">stack</span> {@room.tech_stack}
        </span>
        <span
          :for={tag <- parse_tags(@room.tags)}
          class="inline-flex items-center px-2 py-1 rounded-md bg-gray-700/40 text-gray-400 text-xs"
        >
          #{tag}
        </span>

        <span
          :for={related <- parse_tags(Map.get(@room, :related_rooms, ""))}
          class="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-emerald-500/10 text-emerald-400 text-xs border border-emerald-500/15"
        >
          <span class="hero-link w-3 h-3 opacity-50"></span>
          {related}
        </span>

        <div class="flex items-center gap-2 ml-auto">
          <a
            href={"/rooms/#{@room.id}/export"}
            download={"#{@room.id}.md"}
            class="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-gray-500/10 text-gray-400 text-xs border border-gray-500/15 hover:bg-gray-500/20 transition-colors"
          >
            <span class="hero-arrow-down-tray w-3.5 h-3.5"></span>
            export
          </a>
          <button
            :if={present?(@room.system_prompt)}
            phx-click="toggle_system_prompt"
            aria-label={if @show_system_prompt, do: "Hide system prompt", else: "Show system prompt"}
            aria-expanded={to_string(@show_system_prompt)}
            class="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-amber-500/10 text-amber-400 text-xs border border-amber-500/15 hover:bg-amber-500/20 transition-colors cursor-pointer"
          >
            <span class="text-[10px]"><%= if @show_system_prompt, do: "▼", else: "▶" %></span>
            system prompt
          </button>
        </div>
      </div>

      <%!-- System prompt content --%>
      <div
        :if={@show_system_prompt && present?(@room.system_prompt)}
        class="mt-3 p-3 rounded-lg bg-amber-500/5 border border-amber-500/15 text-sm text-gray-300 council-prose"
      >
        {raw(render_markdown(@room.system_prompt))}
      </div>
    </header>
    """
  end

  # -- Message Bubble --

  attr :msg, :map, required: true

  def message_bubble(assigns) do
    ~H"""
    <div class="message-block group">
      <div class="flex gap-3">
        <div
          class="w-8 h-8 rounded-lg flex items-center justify-center shrink-0 text-[11px] font-bold border mt-0.5"
          style={"background: #{author_hex(@msg.author)}15; border-color: #{author_hex(@msg.author)}30; color: #{author_hex(@msg.author)}"}
        >
          {author_initials(@msg.author)}
        </div>

        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 mb-1.5">
            <span class="text-[13px] font-bold" style={"color: #{author_hex(@msg.author)}"}>
              {@msg.author}
            </span>
            <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-800/80 text-gray-400 border border-gray-700/50">
              <span class={[type_icon(@msg.message_type), "w-3 h-3"]}></span>
              {type_label(@msg.message_type)}
            </span>
            <span
              :if={Map.get(@msg, :pinned, false)}
              class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-bold bg-amber-500/15 text-amber-400 border border-amber-500/30"
            >
              <span class="hero-bookmark-solid w-3 h-3"></span>
              PINNED
            </span>
            <span
              :if={Map.get(@msg, :reply_to, 0) > 0}
              class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-cyan-500/10 text-cyan-400 border border-cyan-500/20"
            >
              <span class="hero-arrow-uturn-left w-3 h-3"></span>
              re: #{Map.get(@msg, :reply_to)}
            </span>
            <span class="text-[10px] text-gray-600 font-mono tabular-nums">
              {format_timestamp(@msg.timestamp)}
            </span>
            <span
              id={"msg-time-#{@msg.id}"}
              phx-hook="RelativeTime"
              data-timestamp={NaiveDateTime.to_iso8601(@msg.timestamp)}
              class="text-[10px] text-gray-700 font-mono opacity-0 group-hover:opacity-100 transition-opacity"
            >
            </span>
          </div>

          <div class={"council-prose text-[0.9rem] leading-relaxed text-gray-200 border-l-2 pl-3 #{author_classes(@msg.author)}"}>
            {raw(render_markdown(@msg.content))}
          </div>
        </div>
      </div>
    </div>
    """
  end

  # -- Summary Block --

  attr :msg, :map, required: true
  attr :collapsed, :boolean, default: false

  def summary_block(assigns) do
    ~H"""
    <div class="summary-block border-l-[3px] border-amber-500/50 bg-gradient-to-r from-amber-500/5 to-transparent rounded-r-lg p-4">
      <button
        phx-click="toggle_summary"
        phx-value-id={@msg.id}
        aria-label={if @collapsed, do: "Expand summary", else: "Collapse summary"}
        aria-expanded={to_string(!@collapsed)}
        class="flex items-center gap-2 w-full text-left mb-2 cursor-pointer group"
      >
        <div class="w-7 h-7 rounded-md bg-amber-500/15 flex items-center justify-center shrink-0 border border-amber-500/20">
          <span class="hero-clipboard-document-list w-4 h-4 text-amber-400"></span>
        </div>
        <span class="text-[11px] font-bold text-amber-400 uppercase tracking-wider">Summary</span>
        <span class="text-[10px] text-gray-600 font-mono">{format_timestamp(@msg.timestamp)}</span>
        <span class="text-gray-600 text-[10px] ml-auto group-hover:text-gray-400 transition-colors">
          <%= if @collapsed, do: "▶ expand", else: "▼ collapse" %>
        </span>
      </button>
      <div class={[
        "council-prose text-sm text-gray-300 ml-9 transition-all",
        if(@collapsed, do: "line-clamp-2 opacity-60", else: "")
      ]}>
        {raw(render_markdown(@msg.content))}
      </div>
    </div>
    """
  end
end
