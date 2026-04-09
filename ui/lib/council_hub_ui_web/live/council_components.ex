defmodule CouncilHubUiWeb.CouncilComponents do
  @moduledoc """
  Function components for the Council Hub UI — Palantir Foundry aesthetic.
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
          do: "bg-sky-500/10 border-sky-500/25",
          else: "border-transparent hover:bg-slate-800/40 hover:border-slate-700/30"
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
        <span class="text-[12px] font-medium text-slate-200 truncate leading-tight">
          {@room.id}
        </span>
        <div class="flex items-center gap-1.5 shrink-0">
          <span :if={@count > 0} class="text-[10px] text-slate-400 font-mono">{@count}</span>
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
        class="text-[11px] text-slate-400 leading-snug mt-0.5 truncate"
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
          class="text-[10px] text-slate-500 font-mono"
        >
        </div>
        <%!-- Participant count --%>
        <span
          :if={@participants > 0}
          class="text-[10px] text-slate-500 font-mono"
          title="Participants"
        >
          {@participants}p
        </span>
        <%!-- Full type breakdown --%>
        <span
          :if={format_type_counts(@type_counts)}
          class="text-[10px] text-slate-400 font-mono"
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
        <%!-- Source node --%>
        <span
          :if={present?(@source_node)}
          class="text-[10px] text-sky-400/60 font-mono"
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

  def room_header(assigns) do
    ~H"""
    <header class="bg-[#0f1629]/90 border-b border-slate-700/40 px-4 py-3 shrink-0 backdrop-blur-sm">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <%!-- Title row --%>
          <div class="flex items-center gap-2 mb-1">
            <h2 class="text-[15px] font-semibold text-slate-100 truncate">{@room.id}</h2>
            <span
              class={["w-2 h-2 rounded-full shrink-0", status_dot_class(@room.status)]}
              title={@room.status}
            >
            </span>
            <span class="text-[10px] text-slate-400 uppercase tracking-wider">{@room.status}</span>
          </div>
          <%!-- Description --%>
          <p
            :if={present?(@room.description)}
            class="text-[12px] text-slate-300 leading-snug"
          >
            {@room.description}
          </p>
          <%!-- Participant badges --%>
          <div :if={@participants != []} class="flex items-center gap-1.5 mt-1.5 flex-wrap">
            <span class="text-[10px] text-slate-500 font-mono uppercase tracking-wider shrink-0">
              participants:
            </span>
            <span
              :for={{author, msg_count} <- @participants}
              class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono border border-slate-700/40"
              style={"color: #{author_hex(author)}; background: #{author_hex(author)}12"}
              title={"#{author}: #{msg_count} messages"}
            >
              {author}
              <span class="text-slate-500 text-[9px]">{msg_count}</span>
            </span>
          </div>
        </div>
        <%!-- Right meta column --%>
        <div class="flex flex-col items-end gap-1 shrink-0 text-[10px] text-slate-400 font-mono">
          <span>{@count} msgs</span>
          <span :if={@room.created_at}>{format_date(@room.created_at)}</span>
          <span :if={@time_range} class="text-slate-500" title="First → last message">
            {format_time_range(elem(@time_range, 0), elem(@time_range, 1))}
          </span>
          <div
            :if={Map.get(@room, :updated_at)}
            id={"header-updated-#{@room.id}"}
            phx-hook="RelativeTime"
            data-timestamp={NaiveDateTime.to_iso8601(Map.get(@room, :updated_at))}
            class="text-slate-500"
          >
          </div>
        </div>
      </div>

      <%!-- Metadata chips --%>
      <div class="flex items-center gap-1.5 mt-2 flex-wrap">
        <span
          :if={present?(@room.project)}
          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-sky-500/10 text-sky-300 text-[10px] border border-sky-500/20"
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
          class="px-1.5 py-0.5 rounded bg-slate-800/60 text-slate-400 text-[10px] border border-slate-700/30"
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
                class="px-1.5 py-0.5 rounded bg-slate-800/50 border border-slate-600/40 text-slate-200 text-[10px] font-mono w-40 focus:outline-none focus:border-sky-500/40"
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
                class="px-1.5 py-0.5 rounded bg-slate-700/40 text-slate-400 text-[10px] hover:bg-slate-700/50 transition-colors cursor-pointer"
              >
                cancel
              </button>
            </form>
          </div>
          <button
            :if={not @editing_tags}
            phx-click="edit_tags"
            title="Edit tags"
            class="px-1.5 py-0.5 rounded text-slate-400 text-[10px] hover:text-slate-200 hover:bg-slate-800/40 transition-colors cursor-pointer"
          >
            tags
          </button>
          <button
            phx-click="check_room_health"
            phx-value-room-id={@room.id}
            title="Run lint"
            class="px-1.5 py-0.5 rounded text-slate-400 text-[10px] hover:text-slate-200 hover:bg-slate-800/40 transition-colors cursor-pointer"
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
            class="px-1.5 py-0.5 rounded text-slate-400 text-[10px] hover:text-slate-200 hover:bg-slate-800/40 transition-colors"
          >
            export
          </a>
          <button
            :if={present?(@room.system_prompt)}
            phx-click="toggle_system_prompt"
            aria-label={if @show_system_prompt, do: "Hide system prompt", else: "Show system prompt"}
            aria-expanded={to_string(@show_system_prompt)}
            class="px-1.5 py-0.5 rounded text-sky-300/80 text-[10px] hover:text-sky-300 hover:bg-sky-500/10 transition-colors cursor-pointer"
          >
            {if @show_system_prompt, do: "- prompt", else: "+ prompt"}
          </button>
        </div>
      </div>

      <%!-- System prompt content --%>
      <div
        :if={@show_system_prompt && present?(@room.system_prompt)}
        class="mt-2 p-3 rounded bg-sky-500/5 border border-sky-500/15 text-[12px] text-slate-300 council-prose"
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
        do: "border-l-2 border-sky-500/50 pl-1 bg-sky-500/[0.03]",
        else: ""
      )
    ]}>
      <div class="flex gap-2.5">
        <%!-- Author avatar --%>
        <div
          class="w-6 h-6 rounded flex items-center justify-center shrink-0 text-[9px] font-semibold mt-0.5"
          style={"background: #{author_hex(@msg.author)}15; color: #{author_hex(@msg.author)}"}
        >
          {author_initials(@msg.author)}
        </div>

        <div class="flex-1 min-w-0">
          <%!-- Header row: author, type, indicators, timestamp --%>
          <div class="flex items-center gap-1.5 mb-1 flex-wrap">
            <span class="text-[12px] font-semibold" style={"color: #{author_hex(@msg.author)}"}>
              {@msg.author}
            </span>
            <span class={[
              "text-[9px] font-medium uppercase tracking-wider px-1.5 py-px rounded",
              type_color(@msg.message_type)
            ]}>
              {type_label(@msg.message_type)}
            </span>
            <span
              :if={Map.get(@msg, :pinned, false)}
              class="text-[9px] font-semibold text-sky-300/90 uppercase tracking-wider"
            >
              PIN
            </span>
            <button
              :if={Map.get(@msg, :reply_to, "") != ""}
              id={"reply-btn-#{@msg.id}"}
              phx-hook="ScrollToMessage"
              data-reply-to={Map.get(@msg, :reply_to, "")}
              type="button"
              class="text-[9px] font-mono text-cyan-300/70 hover:text-cyan-300 transition-colors cursor-pointer"
            >
              re: #{String.slice(Map.get(@msg, :reply_to, ""), 0, 8)}
            </button>
            <span class="text-[10px] text-slate-500 font-mono tabular-nums">
              {format_timestamp(@msg.timestamp)}
            </span>
            <span
              id={"msg-time-#{@msg.id}"}
              phx-hook="RelativeTime"
              data-timestamp={NaiveDateTime.to_iso8601(@msg.timestamp)}
              class="text-[10px] text-slate-500 font-mono opacity-0 group-hover:opacity-100 transition-opacity"
            >
            </span>
            <button
              id={"copy-msg-#{@msg.id}"}
              phx-hook="CopyMessage"
              data-copy={"##{@msg.id} | #{format_timestamp(@msg.timestamp)} | #{@msg.author} (#{@msg.message_type})\n\n#{@msg.content}"}
              class="ml-auto opacity-0 group-hover:opacity-100 transition-opacity p-0.5 rounded text-slate-500 hover:text-slate-300 cursor-pointer"
              title="Copy message"
              type="button"
            >
              <span class="hero-clipboard w-3.5 h-3.5"></span>
            </button>
          </div>

          <%!-- @mention tags --%>
          <div
            :if={parse_mentions(Map.get(@msg, :mentions, "")) != []}
            class="flex items-center gap-1 mb-1 flex-wrap"
          >
            <span
              :for={mention <- parse_mentions(Map.get(@msg, :mentions, ""))}
              class="inline-flex items-center px-1.5 py-px rounded bg-cyan-500/8 text-cyan-300/90 text-[10px] font-mono border border-cyan-500/15"
            >
              @{mention}
            </span>
          </div>

          <%!-- Message content --%>
          <div class={"council-prose text-slate-300 border-l border-slate-600/40 pl-2.5 #{author_prose_class(@msg.author)}"}>
            {raw(render_markdown(@msg.content))}
          </div>

          <%!-- Reactions --%>
          <div class="flex items-center gap-1 mt-1 flex-wrap">
            <button
              :for={{emoji, authors} <- parse_reactions(Map.get(@msg, :reactions))}
              phx-click="react"
              phx-value-message-id={@msg.id}
              phx-value-emoji={emoji}
              class="inline-flex items-center gap-1 px-1.5 py-px rounded bg-slate-800/50 text-[11px] border border-slate-700/30 hover:bg-slate-700/40 active:scale-95 transition-all cursor-pointer"
              title={Enum.join(authors, ", ")}
              type="button"
            >
              {emoji}
              <span class="text-slate-400 text-[10px] font-mono">{length(authors)}</span>
              <span class="text-slate-500 text-[9px] font-mono opacity-0 group-hover:opacity-100 transition-opacity">
                {Enum.join(authors, ", ")}
              </span>
            </button>
            <div
              id={"emoji-picker-#{@msg.id}"}
              phx-hook="EmojiPicker"
              data-message-id={@msg.id}
              class="relative inline-flex opacity-0 group-hover:opacity-100 transition-opacity"
            >
              <button
                type="button"
                class="emoji-picker-trigger inline-flex items-center justify-center w-5 h-5 rounded bg-slate-800/40 border border-slate-700/30 text-slate-500 hover:text-slate-300 transition-colors cursor-pointer text-[10px]"
                title="Add reaction"
              >
                +
              </button>
              <div class="emoji-picker-panel hidden absolute bottom-6 left-0 z-50 flex gap-0.5 p-1 rounded bg-[#0f172a] border border-slate-700/50 shadow-xl">
                <button
                  :for={e <- ~w(👍 ❤️ 🎉 🚀 👀 ✅ 🤔 🔥)}
                  type="button"
                  phx-click="react"
                  phx-value-message-id={@msg.id}
                  phx-value-emoji={e}
                  class="text-sm hover:scale-110 transition-transform cursor-pointer p-0.5 rounded"
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
    <div class="summary-block border-l-2 border-sky-500/40 bg-sky-500/[0.03] rounded-r p-3">
      <button
        phx-click="toggle_summary"
        phx-value-id={@msg.id}
        aria-label={if @collapsed, do: "Expand summary", else: "Collapse summary"}
        aria-expanded={to_string(!@collapsed)}
        class="flex items-center gap-2 w-full text-left mb-2 cursor-pointer group"
      >
        <span class="text-[10px] font-semibold text-sky-300/90 uppercase tracking-[0.1em]">
          Summary
        </span>
        <span class="text-[10px] text-slate-500 font-mono">{format_timestamp(@msg.timestamp)}</span>
        <span class="text-slate-500 text-[10px] ml-auto group-hover:text-slate-300 transition-colors">
          {if @collapsed, do: "+", else: "-"}
        </span>
      </button>
      <div class={[
        "council-prose text-slate-300 transition-all",
        if(@collapsed, do: "line-clamp-2 opacity-60", else: "")
      ]}>
        {raw(render_markdown(@msg.content))}
      </div>
      <div
        :if={parse_reactions(Map.get(@msg, :reactions)) != %{}}
        class="flex items-center gap-1 mt-2 flex-wrap"
      >
        <span
          :for={{emoji, authors} <- parse_reactions(Map.get(@msg, :reactions))}
          class="inline-flex items-center gap-1 px-1.5 py-px rounded bg-slate-800/50 text-[11px] border border-slate-700/30 cursor-default"
          title={Enum.join(authors, ", ")}
        >
          {emoji}
          <span class="text-slate-400 text-[10px] font-mono">{length(authors)}</span>
        </span>
      </div>
    </div>
    """
  end

  attr :mentions, :list, required: true

  def mentions_panel(assigns) do
    ~H"""
    <div :if={@mentions != []} class="px-3 py-2 border-t border-slate-700/40">
      <div class="flex items-center gap-1.5 mb-1.5">
        <span class="text-[10px] font-medium text-slate-400 uppercase tracking-[0.12em]">
          Mentions
        </span>
      </div>
      <div class="space-y-px max-h-32 overflow-y-auto">
        <div :for={m <- @mentions}>
          <.link
            patch={"/rooms/#{m.room_id}"}
            class="block px-2 py-1.5 rounded hover:bg-slate-800/30 transition-colors"
          >
            <div class="flex items-center gap-1 text-[11px]">
              <span class="text-cyan-300/80 font-medium truncate">{m.author}</span>
              <span class="text-slate-500">in</span>
              <span class="text-slate-300 font-mono truncate">{m.room_id}</span>
            </div>
            <div class="text-[10px] text-slate-500 truncate mt-0.5">
              {String.slice(m.content, 0, 60)}
            </div>
          </.link>
        </div>
      </div>
    </div>
    """
  end

  attr :archives, :list, required: true

  def archive_list(assigns) do
    ~H"""
    <div :if={@archives != []} class="px-3 py-2 border-t border-slate-700/40">
      <div class="flex items-center gap-1.5 mb-1.5">
        <span class="text-[10px] font-medium text-slate-400 uppercase tracking-[0.12em]">
          Archives ({length(@archives)})
        </span>
      </div>
      <div class="space-y-px max-h-36 overflow-y-auto">
        <button
          :for={a <- @archives}
          phx-click="view_archive"
          phx-value-room-id={a["room_id"]}
          class="w-full text-left px-2 py-1.5 rounded hover:bg-slate-800/30 transition-colors flex items-center justify-between"
        >
          <span class="text-[11px] font-mono text-slate-300 truncate">{a["room_id"]}</span>
          <span class="text-[10px] text-slate-500 ml-2 shrink-0">
            {if a["archived_at"], do: String.slice(a["archived_at"], 0, 10), else: ""}
          </span>
        </button>
      </div>
    </div>
    """
  end

  attr :active_archive, :map, required: true

  def archive_modal(assigns) do
    ~H"""
    <div
      :if={@active_archive != nil}
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
    >
      <div class="bg-[#131a2e] border border-slate-700/40 rounded w-3/4 max-h-[80vh] overflow-hidden flex flex-col shadow-2xl">
        <div class="flex items-center justify-between px-4 py-2.5 border-b border-slate-700/40 shrink-0">
          <span class="font-mono text-[12px] text-slate-300">
            {@active_archive.room_id} / archive
          </span>
          <button
            phx-click="close_archive"
            class="text-slate-400 hover:text-slate-200 transition-colors"
            aria-label="Close archive"
          >
            <span class="hero-x-mark w-4 h-4"></span>
          </button>
        </div>
        <div class="overflow-y-auto px-5 py-4 council-prose text-slate-300">
          {Phoenix.HTML.raw(CouncilHubUiWeb.CouncilHelpers.render_markdown(@active_archive.content))}
        </div>
      </div>
    </div>
    """
  end
end
