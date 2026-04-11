defmodule CouncilHubUiWeb.MessageComponents do
  @moduledoc "Message bubble and summary block function components."

  use Phoenix.Component
  import Phoenix.HTML, only: [raw: 1]
  import CouncilHubUiWeb.CouncilHelpers

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
end
