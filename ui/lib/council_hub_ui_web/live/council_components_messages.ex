defmodule CouncilHubUiWeb.MessageComponents do
  @moduledoc "Message bubble and summary block function components."

  use Phoenix.Component
  import Phoenix.HTML, only: [raw: 1]
  import CouncilHubUiWeb.CouncilHelpers

  # -- Retraction rendering --
  #
  # Shared by the room view (message_bubble) and the notebook outline so the
  # tombstone markup can't drift between the two.

  @doc "Header badge marking a retracted message."
  attr :retracted_by, :string, default: ""

  def retracted_badge(assigns) do
    ~H"""
    <span
      title={"Retracted#{if @retracted_by != "", do: " by " <> @retracted_by, else: ""} — preserved as a tombstone"}
      class="text-[9px] font-mono uppercase tracking-wider text-[var(--ch-text-lo)]"
    >
      retracted
    </span>
    """
  end

  @doc "Body swap for a retracted message: the tombstone text instead of the content."
  attr :retracted_by, :string, default: ""
  attr :class, :any, default: nil

  def retracted_body(assigns) do
    ~H"""
    <div class={@class}>
      [retracted{if @retracted_by != "", do: " by " <> @retracted_by, else: ""}]
    </div>
    """
  end

  # -- Message Bubble --

  attr :msg, :map, required: true
  attr :repo, :string, default: ""
  attr :compact, :boolean, default: false
  # Prior versions of an edited message (oldest → newest), or nil when collapsed.
  # Loaded on demand by the "✎ edited" toggle.
  attr :revisions, :list, default: nil

  def message_bubble(assigns) do
    ~H"""
    <div class={[
      "message-block group",
      if(Map.get(@msg, :pinned, false),
        do: "border-l-2 border-[var(--ch-border-mid)] pl-1 bg-[var(--ch-raised)]",
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
              class="text-[9px] font-semibold text-[var(--ch-text-mid)] uppercase tracking-wider"
            >
              PIN
            </span>
            <button
              :if={Map.get(@msg, :reply_to, "") != ""}
              id={"reply-btn-#{@msg.id}"}
              phx-hook="ScrollToMessage"
              data-reply-to={Map.get(@msg, :reply_to, "")}
              type="button"
              class="text-[9px] font-mono text-[var(--ch-text-lo)] hover:text-[var(--ch-text-mid)] transition-colors cursor-pointer"
            >
              re: #{String.slice(Map.get(@msg, :reply_to, ""), 0, 8)}
            </button>
            <button
              :if={Map.get(@msg, :supersedes, "") != ""}
              id={"supersedes-btn-#{@msg.id}"}
              phx-hook="ScrollToMessage"
              data-reply-to={Map.get(@msg, :supersedes, "")}
              type="button"
              title="Replaces an earlier message"
              class="text-[9px] font-mono text-[var(--ch-text-lo)] hover:text-[var(--ch-text-mid)] transition-colors cursor-pointer"
            >
              supersedes #{String.slice(Map.get(@msg, :supersedes, ""), 0, 8)}
            </button>
            <button
              :if={Map.get(@msg, :superseded_by, "") != ""}
              id={"superseded-by-btn-#{@msg.id}"}
              phx-hook="ScrollToMessage"
              data-reply-to={Map.get(@msg, :superseded_by, "")}
              type="button"
              title="Replaced by a later message — this version is stale"
              class="text-[9px] font-mono text-amber-400/70 hover:text-amber-300 transition-colors cursor-pointer"
            >
              ⚠ superseded by #{String.slice(Map.get(@msg, :superseded_by, ""), 0, 8)}
            </button>
            <button
              :if={Map.get(@msg, :revises, "") != ""}
              phx-click="toggle_revisions"
              phx-value-id={@msg.id}
              type="button"
              title="Edited — earlier versions are preserved (append-only). Click to show history."
              class="text-[9px] font-mono text-[var(--ch-text-lo)] hover:text-[var(--ch-text-mid)] transition-colors cursor-pointer"
            >
              ✎ edited{if @revisions, do: " ▾", else: " ▸"}
            </button>
            <.retracted_badge
              :if={Map.get(@msg, :retracted_at) != nil}
              retracted_by={Map.get(@msg, :retracted_by, "")}
            />
            <span class="text-[10px] text-[var(--ch-text-xs)] font-mono tabular-nums">
              {format_timestamp(@msg.timestamp)}
            </span>
            <span
              id={"msg-time-#{@msg.id}"}
              phx-hook="RelativeTime"
              data-timestamp={NaiveDateTime.to_iso8601(@msg.timestamp)}
              class="text-[10px] text-[var(--ch-text-xs)] font-mono opacity-0 group-hover:opacity-100 transition-opacity"
            ></span>
            <button
              id={"permalink-#{@msg.id}"}
              phx-hook="CopyMessage"
              data-copy={"council://message/#{@msg.id}"}
              class="ml-auto opacity-0 group-hover:opacity-100 transition-opacity p-0.5 rounded text-[var(--ch-text-xs)] hover:text-[var(--ch-text-mid)] cursor-pointer"
              title={"Copy address — council://message/#{String.slice(@msg.id, 0, 8)}…"}
              type="button"
            >
              <span class="hero-link w-3.5 h-3.5"></span>
            </button>
            <button
              id={"copy-msg-#{@msg.id}"}
              phx-hook="CopyMessage"
              data-copy={copy_text(@msg)}
              class="opacity-0 group-hover:opacity-100 transition-opacity p-0.5 rounded text-[var(--ch-text-xs)] hover:text-[var(--ch-text-mid)] cursor-pointer"
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
              class="inline-flex items-center px-1.5 py-px rounded bg-[var(--ch-raised)] text-[var(--ch-text-mid)] text-[10px] font-mono border border-[var(--ch-border)]"
            >
              @{mention}
            </span>
          </div>

          <%!-- Message content (a retracted node renders as a tombstone, not its text) --%>
          <.retracted_body
            :if={Map.get(@msg, :retracted_at) != nil}
            retracted_by={Map.get(@msg, :retracted_by, "")}
            class="text-[var(--ch-text-lo)] italic border-l border-[var(--ch-border-mid)] pl-2.5"
          />
          <div
            :if={Map.get(@msg, :retracted_at) == nil}
            class={[
              "council-prose text-[var(--ch-text-mid)] border-l border-[var(--ch-border-mid)] pl-2.5 #{author_prose_class(@msg.author)}",
              @compact && "line-clamp-1"
            ]}
          >
            {raw(render_markdown(resolve_commit_refs(@msg.content, @repo)))}
          </div>

          <%!-- Revision history (expanded via ✎ edited) — prior versions, oldest → newest --%>
          <div
            :if={@revisions not in [nil, []]}
            class="mt-1 ml-2.5 border-l-2 border-dashed border-[var(--ch-border)] pl-2.5 space-y-1"
          >
            <div class="text-[9px] uppercase tracking-wider text-[var(--ch-text-xs)]">
              {length(@revisions)} prior version{if length(@revisions) == 1, do: "", else: "s"}
            </div>
            <div :for={{prev, i} <- Enum.with_index(@revisions, 1)} class="text-[11px]">
              <span class="font-mono text-[var(--ch-text-xs)]">
                v{i} · #{String.slice(prev.id, 0, 8)} · {format_timestamp(prev.timestamp)}{if prev.author &&
                                                                                                prev.author !=
                                                                                                  "",
                                                                                              do:
                                                                                                " · " <>
                                                                                                  prev.author,
                                                                                              else: ""}
              </span>
              <div class="text-[var(--ch-text-lo)] line-through opacity-70">
                {raw(render_markdown(resolve_commit_refs(prev.content, @repo)))}
              </div>
            </div>
          </div>

          <%!-- Typed links (E2 graph) — explicit edges to/from this message --%>
          <div
            :if={Map.get(@msg, :links, []) != []}
            class="flex items-center gap-1 mt-1 flex-wrap"
          >
            <span class="text-[9px] text-[var(--ch-text-xs)] uppercase tracking-wider">
              links
            </span>
            <button
              :for={edge <- Map.get(@msg, :links, [])}
              id={"link-#{@msg.id}-#{edge.direction}-#{edge.relation}-#{edge.other_id}"}
              phx-hook="ScrollToMessage"
              data-reply-to={edge.other_id}
              type="button"
              class="inline-flex items-center gap-1 px-1.5 py-px rounded bg-[var(--ch-raised)] border border-[var(--ch-border)] text-[9px] font-mono text-[var(--ch-text-lo)] hover:text-[var(--ch-text-mid)] transition-colors cursor-pointer"
              title={"#{link_arrow(edge.direction)} #{edge.relation} ##{String.slice(edge.other_id, 0, 8)}"}
            >
              {link_arrow(edge.direction)} {edge.relation} #{String.slice(edge.other_id, 0, 8)}
            </button>
          </div>

          <%!-- Reactions --%>
          <div class="flex items-center gap-1 mt-1 flex-wrap">
            <button
              :for={{emoji, authors} <- parse_reactions(Map.get(@msg, :reactions))}
              phx-click="react"
              phx-value-message-id={@msg.id}
              phx-value-emoji={emoji}
              class="inline-flex items-center gap-1 px-1.5 py-px rounded bg-[var(--ch-raised)] text-[11px] border border-[var(--ch-border)] hover:bg-[var(--ch-active-bg)] active:scale-95 transition-all cursor-pointer"
              title={Enum.join(authors, ", ")}
              type="button"
            >
              {emoji}
              <span class="text-[var(--ch-text-lo)] text-[10px] font-mono">{length(authors)}</span>
              <span class="text-[var(--ch-text-xs)] text-[9px] font-mono opacity-0 group-hover:opacity-100 transition-opacity">
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
                class="emoji-picker-trigger inline-flex items-center justify-center w-5 h-5 rounded bg-[var(--ch-raised)] border border-[var(--ch-border)] text-[var(--ch-text-xs)] hover:text-[var(--ch-text-mid)] transition-colors cursor-pointer text-[10px]"
                title="Add reaction"
              >
                +
              </button>
              <div class="emoji-picker-panel hidden absolute bottom-6 left-0 z-50 flex gap-0.5 p-1 rounded bg-[var(--ch-float)] border border-[var(--ch-border)] shadow-xl">
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

  # Arrow for a typed link: outgoing (this → other) vs incoming (other → this).
  defp link_arrow(:out), do: "→"
  defp link_arrow(_), do: "←"

  # Clipboard payload for the copy-message button. A retracted node is a
  # tombstone: its display is swapped for [retracted], and so is what you can
  # copy — the raw content must not ride out through the data-copy attribute.
  defp copy_text(msg) do
    body =
      if Map.get(msg, :retracted_at) != nil do
        by = Map.get(msg, :retracted_by, "")
        "[retracted#{if by != "", do: " by " <> by, else: ""}]"
      else
        msg.content
      end

    "##{msg.id} | #{format_timestamp(msg.timestamp)} | #{msg.author} (#{msg.message_type})\n\n#{body}"
  end

  # -- Summary Block --

  attr :msg, :map, required: true
  attr :collapsed, :boolean, default: false
  attr :repo, :string, default: ""

  def summary_block(assigns) do
    ~H"""
    <div class="summary-block border-l-2 border-[var(--ch-border-mid)] bg-[var(--ch-raised)] rounded-r p-3">
      <button
        phx-click="toggle_summary"
        phx-value-id={@msg.id}
        aria-label={if @collapsed, do: "Expand summary", else: "Collapse summary"}
        aria-expanded={to_string(!@collapsed)}
        class="flex items-center gap-2 w-full text-left mb-2 cursor-pointer group"
      >
        <span class="text-[10px] font-semibold text-[var(--ch-text-lo)] uppercase tracking-[0.1em]">
          Summary
        </span>
        <span class="text-[10px] text-[var(--ch-text-xs)] font-mono">
          {format_timestamp(@msg.timestamp)}
        </span>
        <span class="text-[var(--ch-text-xs)] text-[10px] ml-auto group-hover:text-[var(--ch-text-mid)] transition-colors">
          {if @collapsed, do: "+", else: "-"}
        </span>
      </button>
      <div class={[
        "council-prose text-[var(--ch-text-mid)] transition-all",
        if(@collapsed, do: "line-clamp-2 opacity-60", else: "")
      ]}>
        {raw(render_markdown(resolve_commit_refs(@msg.content, @repo)))}
      </div>
      <div
        :if={parse_reactions(Map.get(@msg, :reactions)) != %{}}
        class="flex items-center gap-1 mt-2 flex-wrap"
      >
        <span
          :for={{emoji, authors} <- parse_reactions(Map.get(@msg, :reactions))}
          class="inline-flex items-center gap-1 px-1.5 py-px rounded bg-[var(--ch-raised)] text-[11px] border border-[var(--ch-border)] cursor-default"
          title={Enum.join(authors, ", ")}
        >
          {emoji}
          <span class="text-[var(--ch-text-lo)] text-[10px] font-mono">{length(authors)}</span>
        </span>
      </div>
    </div>
    """
  end
end
