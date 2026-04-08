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

  def room_card(assigns) do
    ~H"""
    <.link
      patch={"/rooms/#{@room.id}"}
      class={[
        "block px-2 py-1.5 rounded transition-all duration-100 group border",
        if(@active,
          do: "bg-sky-500/8 border-sky-500/20",
          else: "border-transparent hover:bg-slate-800/30 hover:border-slate-700/20"
        ),
        if(room_health_flags(@room).stale,
          do: "border-l-2 border-l-red-500/40",
          else:
            if(room_health_flags(@room).needs_synthesis,
              do: "border-l-2 border-l-amber-500/40",
              else: nil
            )
        )
      ]}
    >
      <div class="flex items-center justify-between gap-1.5">
        <span class="text-[11px] font-medium text-slate-300 truncate leading-tight">
          {@room.id}
        </span>
        <div class="flex items-center gap-1 shrink-0">
          <span :if={@count > 0} class="text-[9px] text-slate-500 font-mono">{@count}</span>
          <span
            :if={@compiled}
            class="text-[9px] text-purple-400/70"
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
      <p
        :if={present?(@room.description)}
        class="text-[10px] text-slate-500 leading-snug mt-0.5 truncate"
      >
        {@room.description}
      </p>
      <div class="flex items-center gap-2 mt-0.5">
        <div
          :if={@room.updated_at}
          id={"room-time-#{@room.id}"}
          phx-hook="RelativeTime"
          data-timestamp={NaiveDateTime.to_iso8601(@room.updated_at)}
          class="text-[9px] text-slate-600 font-mono"
        >
        </div>
        <span
          :if={Map.get(@type_counts, "decision", 0) > 0 or Map.get(@type_counts, "action", 0) > 0}
          class="text-[9px] text-slate-600 font-mono"
        >
          {Map.get(@type_counts, "decision", 0)}d {Map.get(@type_counts, "action", 0)}a
        </span>
        <span
          :if={present?(@source_node)}
          class="text-[8px] text-sky-500/60 font-mono"
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

  def room_header(assigns) do
    ~H"""
    <header class="bg-[#0a0f1a]/80 border-b border-slate-800/40 px-4 py-2.5 shrink-0 backdrop-blur-sm">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 mb-0.5">
            <h2 class="text-sm font-semibold text-slate-200 truncate">{@room.id}</h2>
            <span
              class={["w-1.5 h-1.5 rounded-full shrink-0", status_dot_class(@room.status)]}
              title={@room.status}
            >
            </span>
            <span class="text-[9px] text-slate-500 uppercase tracking-wider">{@room.status}</span>
          </div>
          <p
            :if={present?(@room.description)}
            class="text-[11px] text-slate-400 leading-snug"
          >
            {@room.description}
          </p>
        </div>
        <div class="flex items-center gap-3 shrink-0 text-[9px] text-slate-500 font-mono">
          <span>{@count} msgs</span>
          <span :if={@room.created_at}>{format_date(@room.created_at)}</span>
          <div
            :if={Map.get(@room, :updated_at)}
            id={"header-updated-#{@room.id}"}
            phx-hook="RelativeTime"
            data-timestamp={NaiveDateTime.to_iso8601(Map.get(@room, :updated_at))}
          >
          </div>
        </div>
      </div>

      <%!-- Metadata --%>
      <div class="flex items-center gap-1.5 mt-2 flex-wrap">
        <span
          :if={present?(@room.project)}
          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-sky-500/8 text-sky-400/80 text-[9px] border border-sky-500/10"
        >
          {String.upcase(@room.project)}
        </span>
        <span
          :if={present?(@room.tech_stack)}
          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-purple-500/8 text-purple-400/70 text-[9px] border border-purple-500/10"
        >
          {String.upcase(@room.tech_stack)}
        </span>
        <span
          :for={tag <- parse_tags(@room.tags)}
          class="px-1.5 py-0.5 rounded bg-slate-800/40 text-slate-500 text-[9px]"
        >
          {tag}
        </span>

        <.link
          :for={related <- parse_tags(Map.get(@room, :related_rooms, ""))}
          patch={"/rooms/#{related}"}
          class="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-emerald-500/6 text-emerald-400/70 text-[9px] border border-emerald-500/10 hover:bg-emerald-500/12 transition-colors cursor-pointer"
        >
          <span class="hero-link w-2.5 h-2.5 opacity-50"></span>
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
                class="px-1.5 py-0.5 rounded bg-slate-800/50 border border-slate-600/30 text-slate-200 text-[10px] font-mono w-40 focus:outline-none focus:border-sky-500/30"
              />
              <button
                type="submit"
                class="px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400/80 text-[9px] border border-emerald-500/15 hover:bg-emerald-500/15 transition-colors cursor-pointer"
              >
                save
              </button>
              <button
                type="button"
                phx-click="cancel_edit_tags"
                class="px-1.5 py-0.5 rounded bg-slate-700/30 text-slate-400 text-[9px] hover:bg-slate-700/40 transition-colors cursor-pointer"
              >
                cancel
              </button>
            </form>
          </div>
          <button
            :if={not @editing_tags}
            phx-click="edit_tags"
            title="Edit tags"
            class="px-1.5 py-0.5 rounded text-slate-500 text-[9px] hover:text-slate-300 hover:bg-slate-800/30 transition-colors cursor-pointer"
          >
            tags
          </button>
          <button
            phx-click="check_room_health"
            phx-value-room-id={@room.id}
            title="Run lint"
            class="px-1.5 py-0.5 rounded text-slate-500 text-[9px] hover:text-slate-300 hover:bg-slate-800/30 transition-colors cursor-pointer"
          >
            lint
          </button>
          <button
            :if={@room.status == "resolved"}
            phx-click="archive_room"
            phx-value-room-id={@room.id}
            title="Archive"
            class="px-1.5 py-0.5 rounded text-red-400/70 text-[9px] hover:text-red-400 hover:bg-red-500/10 transition-colors cursor-pointer"
          >
            archive
          </button>
          <a
            href={"/rooms/#{@room.id}/export"}
            download={"#{@room.id}.md"}
            class="px-1.5 py-0.5 rounded text-slate-500 text-[9px] hover:text-slate-300 hover:bg-slate-800/30 transition-colors"
          >
            export
          </a>
          <button
            :if={present?(@room.system_prompt)}
            phx-click="toggle_system_prompt"
            aria-label={if @show_system_prompt, do: "Hide system prompt", else: "Show system prompt"}
            aria-expanded={to_string(@show_system_prompt)}
            class="px-1.5 py-0.5 rounded text-sky-400/70 text-[9px] hover:text-sky-400 hover:bg-sky-500/10 transition-colors cursor-pointer"
          >
            {if @show_system_prompt, do: "- prompt", else: "+ prompt"}
          </button>
        </div>
      </div>

      <%!-- System prompt content --%>
      <div
        :if={@show_system_prompt && present?(@room.system_prompt)}
        class="mt-2 p-2.5 rounded bg-sky-500/3 border border-sky-500/10 text-[11px] text-slate-300 council-prose"
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
        do: "border-l-2 border-sky-500/40 pl-1 bg-sky-500/[0.02]",
        else: ""
      )
    ]}>
      <div class="flex gap-2">
        <div
          class="w-5 h-5 rounded flex items-center justify-center shrink-0 text-[8px] font-semibold mt-0.5"
          style={"background: #{author_hex(@msg.author)}10; color: #{author_hex(@msg.author)}"}
        >
          {author_initials(@msg.author)}
        </div>

        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-1.5 mb-1">
            <span class="text-[11px] font-medium" style={"color: #{author_hex(@msg.author)}"}>
              {@msg.author}
            </span>
            <span class={[
              "text-[8px] font-medium uppercase tracking-wider px-1 py-px rounded",
              type_color(@msg.message_type)
            ]}>
              {type_label(@msg.message_type)}
            </span>
            <span
              :if={Map.get(@msg, :pinned, false)}
              class="text-[8px] font-semibold text-sky-400/80 uppercase tracking-wider"
            >
              PIN
            </span>
            <button
              :if={Map.get(@msg, :reply_to, "") != ""}
              id={"reply-btn-#{@msg.id}"}
              phx-hook="ScrollToMessage"
              data-reply-to={Map.get(@msg, :reply_to, "")}
              type="button"
              class="text-[8px] font-mono text-cyan-400/60 hover:text-cyan-400 transition-colors cursor-pointer"
            >
              re: #{String.slice(Map.get(@msg, :reply_to, ""), 0, 8)}
            </button>
            <span class="text-[9px] text-slate-600 font-mono tabular-nums">
              {format_timestamp(@msg.timestamp)}
            </span>
            <span
              id={"msg-time-#{@msg.id}"}
              phx-hook="RelativeTime"
              data-timestamp={NaiveDateTime.to_iso8601(@msg.timestamp)}
              class="text-[9px] text-slate-700 font-mono opacity-0 group-hover:opacity-100 transition-opacity"
            >
            </span>
            <button
              id={"copy-msg-#{@msg.id}"}
              phx-hook="CopyMessage"
              data-copy={"##{@msg.id} | #{format_timestamp(@msg.timestamp)} | #{@msg.author} (#{@msg.message_type})\n\n#{@msg.content}"}
              class="ml-auto opacity-0 group-hover:opacity-100 transition-opacity p-0.5 rounded text-slate-600 hover:text-slate-300 cursor-pointer"
              title="Copy message"
              type="button"
            >
              <span class="hero-clipboard w-3 h-3"></span>
            </button>
          </div>

          <div class={"council-prose text-slate-300 border-l border-slate-700/30 pl-2.5 #{author_prose_class(@msg.author)}"}>
            {raw(render_markdown(@msg.content))}
          </div>
          <div class="flex items-center gap-1 mt-1 flex-wrap">
            <button
              :for={{emoji, authors} <- parse_reactions(Map.get(@msg, :reactions))}
              phx-click="react"
              phx-value-message-id={@msg.id}
              phx-value-emoji={emoji}
              class="inline-flex items-center gap-0.5 px-1.5 py-px rounded bg-slate-800/40 text-[10px] border border-slate-700/20 hover:bg-slate-700/30 active:scale-95 transition-all cursor-pointer"
              title={Enum.join(authors, ", ")}
              type="button"
            >
              {emoji} <span class="text-slate-500 text-[9px] font-mono">{length(authors)}</span>
            </button>
            <div
              id={"emoji-picker-#{@msg.id}"}
              phx-hook="EmojiPicker"
              data-message-id={@msg.id}
              class="relative inline-flex opacity-0 group-hover:opacity-100 transition-opacity"
            >
              <button
                type="button"
                class="emoji-picker-trigger inline-flex items-center justify-center w-4 h-4 rounded bg-slate-800/30 border border-slate-700/20 text-slate-600 hover:text-slate-300 transition-colors cursor-pointer text-[9px]"
                title="Add reaction"
              >
                +
              </button>
              <div class="emoji-picker-panel hidden absolute bottom-6 left-0 z-50 flex gap-0.5 p-1 rounded bg-[#0f172a] border border-slate-700/40 shadow-xl">
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
    <div class="summary-block border-l-2 border-sky-500/30 bg-sky-500/[0.02] rounded-r p-2.5">
      <button
        phx-click="toggle_summary"
        phx-value-id={@msg.id}
        aria-label={if @collapsed, do: "Expand summary", else: "Collapse summary"}
        aria-expanded={to_string(!@collapsed)}
        class="flex items-center gap-2 w-full text-left mb-1.5 cursor-pointer group"
      >
        <span class="text-[9px] font-semibold text-sky-400/80 uppercase tracking-[0.1em]">
          Summary
        </span>
        <span class="text-[9px] text-slate-600 font-mono">{format_timestamp(@msg.timestamp)}</span>
        <span class="text-slate-600 text-[9px] ml-auto group-hover:text-slate-400 transition-colors">
          {if @collapsed, do: "+", else: "-"}
        </span>
      </button>
      <div class={[
        "council-prose text-[12px] text-slate-300 transition-all",
        if(@collapsed, do: "line-clamp-2 opacity-50", else: "")
      ]}>
        {raw(render_markdown(@msg.content))}
      </div>
      <div
        :if={parse_reactions(Map.get(@msg, :reactions)) != %{}}
        class="flex items-center gap-1 mt-1.5 flex-wrap"
      >
        <span
          :for={{emoji, authors} <- parse_reactions(Map.get(@msg, :reactions))}
          class="inline-flex items-center gap-0.5 px-1.5 py-px rounded bg-slate-800/40 text-[10px] border border-slate-700/20 cursor-default"
          title={Enum.join(authors, ", ")}
        >
          {emoji} <span class="text-slate-500 text-[9px] font-mono">{length(authors)}</span>
        </span>
      </div>
    </div>
    """
  end

  attr :mentions, :list, required: true

  def mentions_panel(assigns) do
    ~H"""
    <div :if={@mentions != []} class="px-3 py-1.5 border-t border-slate-800/30">
      <div class="flex items-center gap-1.5 mb-1">
        <span class="text-[9px] font-medium text-slate-500 uppercase tracking-[0.12em]">
          Mentions
        </span>
      </div>
      <div class="space-y-px max-h-32 overflow-y-auto">
        <div :for={m <- @mentions}>
          <.link
            patch={"/rooms/#{m.room_id}"}
            class="block px-2 py-1 rounded hover:bg-slate-800/20 transition-colors"
          >
            <div class="flex items-center gap-1 text-[10px]">
              <span class="text-cyan-400/70 font-medium truncate">{m.author}</span>
              <span class="text-slate-600">in</span>
              <span class="text-slate-400 font-mono truncate">{m.room_id}</span>
            </div>
            <div class="text-[9px] text-slate-600 truncate">
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
    <div :if={@archives != []} class="px-3 py-1.5 border-t border-slate-800/30">
      <div class="flex items-center gap-1.5 mb-1">
        <span class="text-[9px] font-medium text-slate-500 uppercase tracking-[0.12em]">
          Archives ({length(@archives)})
        </span>
      </div>
      <div class="space-y-px max-h-36 overflow-y-auto">
        <button
          :for={a <- @archives}
          phx-click="view_archive"
          phx-value-room-id={a["room_id"]}
          class="w-full text-left px-2 py-1 rounded hover:bg-slate-800/20 transition-colors flex items-center justify-between"
        >
          <span class="text-[10px] font-mono text-slate-400 truncate">{a["room_id"]}</span>
          <span class="text-[9px] text-slate-600 ml-2 shrink-0">
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
      <div class="bg-[#0c1120] border border-slate-700/30 rounded w-3/4 max-h-[80vh] overflow-hidden flex flex-col shadow-2xl">
        <div class="flex items-center justify-between px-4 py-2 border-b border-slate-800/40 shrink-0">
          <span class="font-mono text-[11px] text-slate-400">
            {@active_archive.room_id} / archive
          </span>
          <button
            phx-click="close_archive"
            class="text-slate-500 hover:text-slate-300 transition-colors"
            aria-label="Close archive"
          >
            <span class="hero-x-mark w-3.5 h-3.5"></span>
          </button>
        </div>
        <div class="overflow-y-auto px-5 py-3 council-prose text-slate-300">
          {Phoenix.HTML.raw(CouncilHubUiWeb.CouncilHelpers.render_markdown(@active_archive.content))}
        </div>
      </div>
    </div>
    """
  end
end
