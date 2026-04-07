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
  attr :participants, :integer, default: 0
  attr :source_node, :string, default: nil
  attr :latest_id, :string, default: nil

  def room_card(assigns) do
    ~H"""
    <.link
      patch={"/rooms/#{@room.id}"}
      class={[
        "block p-2.5 rounded-lg transition-all duration-150 group border",
        if(@active,
          do: "bg-amber-500/10 border-amber-500/30 shadow-sm shadow-amber-500/5",
          else: "border-transparent hover:bg-zinc-800/40 hover:border-zinc-700/30"
        ),
        if(room_health_flags(@room).stale,
          do: "border-l-2 border-l-red-500/50",
          else:
            if(room_health_flags(@room).needs_synthesis,
              do: "border-l-2 border-l-yellow-500/50",
              else: nil
            )
        )
      ]}
    >
      <div class="flex items-start justify-between gap-2 mb-1">
        <span class="text-[13px] font-semibold text-zinc-200 truncate font-mono leading-tight">
          {@room.id}
        </span>
        <div class="flex items-center gap-1.5 shrink-0">
          <span :if={@count > 0} class="text-[10px] text-zinc-500 font-mono">{@count}</span>
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
        class="text-xs text-zinc-400 leading-relaxed mb-1.5 line-clamp-2"
      >
        {@room.description}
      </p>
      <div :if={parse_tags(@room.tags) != []} class="flex items-center gap-1 flex-wrap">
        <span
          :for={tag <- Enum.take(parse_tags(@room.tags), 3)}
          class="inline-flex items-center px-1.5 py-0.5 rounded-md bg-zinc-700/40 text-zinc-500 text-[10px]"
        >
          {tag}
        </span>
      </div>
      <div
        :if={room_health_flags(@room).stale or room_health_flags(@room).needs_synthesis}
        class="flex items-center gap-1 mt-1"
      >
        <span
          :if={room_health_flags(@room).stale}
          class="inline-flex items-center px-1.5 py-0.5 rounded-md bg-red-500/15 text-red-400 text-[9px] font-medium border border-red-500/20"
        >
          stale
        </span>
        <span
          :if={room_health_flags(@room).needs_synthesis}
          class="inline-flex items-center px-1.5 py-0.5 rounded-md bg-yellow-500/15 text-yellow-400 text-[9px] font-medium border border-yellow-500/20"
        >
          needs synthesis
        </span>
      </div>
      <span
        :if={present?(@source_node)}
        class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 border border-blue-500/20 text-[9px] font-mono mt-1"
      >
        <span class="w-1 h-1 rounded-full bg-blue-400"></span>
        {short_node(@source_node)}
      </span>
      <div class="mt-1.5 flex items-center justify-between gap-2">
        <div
          :if={@room.updated_at}
          id={"room-time-#{@room.id}"}
          phx-hook="RelativeTime"
          data-timestamp={NaiveDateTime.to_iso8601(@room.updated_at)}
          class="text-[10px] text-zinc-600 font-mono"
        >
        </div>
        <span :if={@participants > 1} class="text-[9px] text-zinc-700 font-mono shrink-0">
          {@participants} agents
        </span>
        <span
          :if={present?(@latest_id)}
          class="text-[9px] text-zinc-700 font-mono shrink-0 truncate"
          title={"cursor: #{@latest_id}"}
        >
          {String.slice(@latest_id || "", 0, 8)}
        </span>
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
    <header class="bg-zinc-900/50 border-b border-zinc-800/60 px-6 py-4 shrink-0 backdrop-blur-sm">
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-3 mb-1">
            <h2 class="text-lg font-bold text-zinc-100 font-mono truncate">{@room.id}</h2>
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
            class="text-sm text-zinc-300 leading-relaxed"
          >
            {@room.description}
          </p>
        </div>
        <div class="flex flex-col items-end gap-1 shrink-0 text-right">
          <span class="text-xs text-zinc-500 font-mono tabular-nums">{@count} msgs</span>
          <span :if={@room.created_at} class="text-[10px] text-zinc-600 font-mono">
            {format_date(@room.created_at)}
          </span>
          <div
            :if={Map.get(@room, :updated_at)}
            id={"header-updated-#{@room.id}"}
            phx-hook="RelativeTime"
            data-timestamp={NaiveDateTime.to_iso8601(Map.get(@room, :updated_at))}
            class="text-[10px] text-zinc-600 font-mono"
          >
          </div>
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
          class="inline-flex items-center px-2 py-1 rounded-md bg-zinc-700/40 text-zinc-400 text-xs"
        >
          #{tag}
        </span>

        <.link
          :for={related <- parse_tags(Map.get(@room, :related_rooms, ""))}
          patch={"/rooms/#{related}"}
          class="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-emerald-500/10 text-emerald-400 text-xs border border-emerald-500/15 hover:bg-emerald-500/20 hover:border-emerald-500/25 transition-colors cursor-pointer"
        >
          <span class="hero-link w-3 h-3 opacity-50"></span>
          {related}
        </.link>

        <div class="flex items-center gap-2 ml-auto">
          <a
            href={"/rooms/#{@room.id}/export"}
            download={"#{@room.id}.md"}
            class="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-500/10 text-zinc-400 text-xs border border-zinc-500/15 hover:bg-zinc-500/20 transition-colors"
          >
            <span class="hero-arrow-down-tray w-3.5 h-3.5"></span> export
          </a>
          <button
            :if={present?(@room.system_prompt)}
            phx-click="toggle_system_prompt"
            aria-label={if @show_system_prompt, do: "Hide system prompt", else: "Show system prompt"}
            aria-expanded={to_string(@show_system_prompt)}
            class="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-amber-500/10 text-amber-400 text-xs border border-amber-500/15 hover:bg-amber-500/20 transition-colors cursor-pointer"
          >
            <span class="text-[10px]">{if @show_system_prompt, do: "▼", else: "▶"}</span>
            system prompt
          </button>
        </div>
      </div>

      <%!-- System prompt content --%>
      <div
        :if={@show_system_prompt && present?(@room.system_prompt)}
        class="mt-3 p-3 rounded-lg bg-amber-500/5 border border-amber-500/15 text-sm text-zinc-300 council-prose"
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
    <div class={[
      "message-block group",
      if(Map.get(@msg, :pinned, false),
        do: "border-l-2 border-amber-500/50 pl-1 bg-amber-500/[0.03] rounded-r-lg",
        else: ""
      )
    ]}>
      <div class="flex gap-3">
        <div
          class="w-8 h-8 rounded-xl flex items-center justify-center shrink-0 text-[11px] font-bold border mt-0.5"
          style={"background: #{author_hex(@msg.author)}15; border-color: #{author_hex(@msg.author)}30; color: #{author_hex(@msg.author)}"}
        >
          {author_initials(@msg.author)}
        </div>

        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 mb-1.5">
            <span class="text-[13px] font-bold" style={"color: #{author_hex(@msg.author)}"}>
              {@msg.author}
            </span>
            <span class={[
              "inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium border",
              type_color(@msg.message_type)
            ]}>
              <span class={[type_icon(@msg.message_type), "w-3 h-3"]}></span>
              {type_label(@msg.message_type)}
            </span>
            <span
              :if={Map.get(@msg, :pinned, false)}
              class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-bold bg-amber-500/15 text-amber-400 border border-amber-500/30"
            >
              <span class="hero-bookmark-solid w-3 h-3"></span> PINNED
            </span>
            <span
              :if={Map.get(@msg, :reply_to, "") != ""}
              class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-cyan-500/10 text-cyan-400 border border-cyan-500/20"
            >
              <span class="hero-arrow-uturn-left w-3 h-3"></span>
              re: #{String.slice(Map.get(@msg, :reply_to, ""), 0, 8)}
            </span>
            <span class="text-[10px] text-zinc-600 font-mono tabular-nums">
              {format_timestamp(@msg.timestamp)}
            </span>
            <span
              id={"msg-time-#{@msg.id}"}
              phx-hook="RelativeTime"
              data-timestamp={NaiveDateTime.to_iso8601(@msg.timestamp)}
              class="text-[10px] text-zinc-700 font-mono opacity-0 group-hover:opacity-100 transition-opacity"
            >
            </span>
            <button
              id={"copy-msg-#{@msg.id}"}
              phx-hook="CopyMessage"
              data-copy={"##{@msg.id} | #{format_timestamp(@msg.timestamp)} | #{@msg.author} (#{@msg.message_type})\n\n#{@msg.content}"}
              class="ml-auto opacity-0 group-hover:opacity-100 transition-opacity p-0.5 rounded text-zinc-600 hover:text-zinc-300 hover:bg-white/5 cursor-pointer"
              title="Copy message"
              type="button"
            >
              <span class="hero-clipboard w-3.5 h-3.5"></span>
            </button>
          </div>

          <div class={"council-prose text-[0.9rem] leading-relaxed text-zinc-200 border-l-2 pl-3 #{author_classes(@msg.author)}"}>
            {raw(render_markdown(@msg.content))}
          </div>
          <div class="flex items-center gap-1.5 mt-2 flex-wrap">
            <button
              :for={{emoji, authors} <- parse_reactions(Map.get(@msg, :reactions))}
              phx-click="react"
              phx-value-message-id={@msg.id}
              phx-value-emoji={emoji}
              class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-zinc-800/80 text-xs border border-zinc-700/50 hover:bg-zinc-700/60 active:scale-95 transition-all cursor-pointer"
              title={Enum.join(authors, ", ")}
              type="button"
            >
              {emoji} <span class="text-zinc-400 text-[10px] font-mono">{length(authors)}</span>
            </button>
            <div
              id={"emoji-picker-#{@msg.id}"}
              phx-hook="EmojiPicker"
              data-message-id={@msg.id}
              class="relative inline-flex opacity-0 group-hover:opacity-100 transition-opacity"
            >
              <button
                type="button"
                class="emoji-picker-trigger inline-flex items-center justify-center w-5 h-5 rounded-full bg-zinc-800/60 border border-zinc-700/40 text-zinc-600 hover:text-zinc-300 hover:border-zinc-500/60 transition-colors cursor-pointer text-[10px]"
                title="Add reaction"
              >
                +
              </button>
              <div class="emoji-picker-panel hidden absolute bottom-7 left-0 z-50 flex gap-1 p-1.5 rounded-lg bg-zinc-900 border border-zinc-700/60 shadow-xl">
                <button
                  :for={e <- ~w(👍 ❤️ 🎉 🚀 👀 ✅ 🤔 🔥)}
                  type="button"
                  phx-click="react"
                  phx-value-message-id={@msg.id}
                  phx-value-emoji={e}
                  class="text-base hover:scale-125 transition-transform cursor-pointer p-0.5 rounded"
                >
                  {e}
                </button>
              </div>
            </div>
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
        <span class="text-[10px] text-zinc-600 font-mono">{format_timestamp(@msg.timestamp)}</span>
        <span class="text-zinc-600 text-[10px] ml-auto group-hover:text-zinc-400 transition-colors">
          {if @collapsed, do: "▶ expand", else: "▼ collapse"}
        </span>
      </button>
      <div class={[
        "council-prose text-sm text-zinc-300 ml-9 transition-all",
        if(@collapsed, do: "line-clamp-2 opacity-60", else: "")
      ]}>
        {raw(render_markdown(@msg.content))}
      </div>
      <div
        :if={parse_reactions(Map.get(@msg, :reactions)) != %{}}
        class="flex items-center gap-1.5 mt-2 ml-9 flex-wrap"
      >
        <span
          :for={{emoji, authors} <- parse_reactions(Map.get(@msg, :reactions))}
          class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-zinc-800/80 text-xs border border-zinc-700/50 hover:bg-zinc-700/60 transition-colors cursor-default"
          title={Enum.join(authors, ", ")}
        >
          {emoji} <span class="text-zinc-400 text-[10px] font-mono">{length(authors)}</span>
        </span>
      </div>
    </div>
    """
  end
end
