defmodule CouncilHubUiWeb.PanelComponents do
  @moduledoc "Sidebar panel function components: mentions, archives, archive modal."

  use Phoenix.Component

  attr :agents, :list, required: true

  @doc """
  Agent rows from `CouncilHubUi.AgentPresence.merge/2`: live connections first
  (green dot, "live", connection count, rooms, client, node when several nodes
  report), then agents with no live connection as "last seen <time> via <tool>".
  Renders nothing when empty.
  """
  def agents_panel(assigns) do
    assigns =
      assign(
        assigns,
        :multi_node,
        assigns.agents |> Enum.flat_map(& &1.nodes) |> Enum.uniq() |> length() > 1
      )

    ~H"""
    <div :if={@agents != []} id="agents-panel" class="px-3 py-2 border-t border-[var(--ch-border)]">
      <div class="flex items-center gap-1.5 mb-1.5">
        <span
          class="text-[10px] font-medium text-[var(--ch-text-lo)] uppercase tracking-[0.12em]"
          title="Live: connections reported by the client, not verified. Last seen: tool calls this node received in the last 24h."
        >
          Agents
        </span>
      </div>
      <div class="space-y-px max-h-40 overflow-y-auto">
        <div
          :for={a <- @agents}
          class="px-2 py-0.5 text-[11px]"
          data-agent={a.agent}
          data-live={to_string(a.live)}
        >
          <div
            class="flex items-center gap-1.5"
            title={
              if a.live,
                do: "Reported by the client, not verified",
                else: "Last tool call #{Calendar.strftime(a.last_seen_at, "%Y-%m-%d %H:%M:%S")} UTC"
            }
          >
            <span
              :if={a.live}
              class="inline-block w-1.5 h-1.5 rounded-full bg-green-500 shrink-0"
            ></span>
            <span class="text-[var(--ch-text-hi)] font-medium truncate">{a.agent}</span>
            <%= if a.live do %>
              <span class="text-[10px] text-green-500 font-mono shrink-0">
                live{if a.count > 1, do: " x#{a.count}"}
              </span>
              <span :if={a.clients != []} class="text-[var(--ch-text-xs)] font-mono truncate">
                {Enum.join(a.clients, ", ")}
              </span>
              <span
                :if={@multi_node and a.nodes != []}
                class="ml-auto text-[10px] text-[var(--ch-text-xs)] font-mono shrink-0"
              >
                {Enum.join(a.nodes, ", ")}
              </span>
            <% else %>
              <span class="text-[var(--ch-text-xs)] font-mono truncate">{a.last_tool}</span>
              <span class="ml-auto text-[10px] text-[var(--ch-text-xs)] font-mono shrink-0">
                last seen {CouncilHubUiWeb.CouncilHelpers.relative_time(a.last_seen_at)}
              </span>
            <% end %>
          </div>
          <div
            :if={a.live and a.rooms != []}
            class="pl-3 text-[10px] text-[var(--ch-text-xs)] font-mono truncate"
            title={"Watching: " <> Enum.join(a.rooms, ", ")}
          >
            watching {Enum.join(a.rooms, ", ")}
          </div>
        </div>
      </div>
    </div>
    """
  end

  attr :mentions, :list, required: true

  def mentions_panel(assigns) do
    ~H"""
    <div :if={@mentions != []} class="px-3 py-2 border-t border-[var(--ch-border)]">
      <div class="flex items-center gap-1.5 mb-1.5">
        <span class="text-[10px] font-medium text-[var(--ch-text-lo)] uppercase tracking-[0.12em]">
          Mentions
        </span>
      </div>
      <div class="space-y-px max-h-32 overflow-y-auto">
        <div :for={m <- @mentions}>
          <.link
            patch={"/rooms/#{m.room_id}"}
            class="block px-2 py-1.5 rounded hover:bg-[var(--ch-hover-bg)] transition-colors"
          >
            <div class="flex items-center gap-1 text-[11px]">
              <span class="text-[var(--ch-text-hi)] font-medium truncate">{m.author}</span>
              <span class="text-[var(--ch-text-xs)]">in</span>
              <span class="text-[var(--ch-text-mid)] font-mono truncate">{m.room_id}</span>
            </div>
            <div class="text-[10px] text-[var(--ch-text-xs)] truncate mt-0.5">
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
    <div :if={@archives != []} class="px-3 py-2 border-t border-[var(--ch-border)]">
      <div class="flex items-center gap-1.5 mb-1.5">
        <span class="text-[10px] font-medium text-[var(--ch-text-lo)] uppercase tracking-[0.12em]">
          Archives ({length(@archives)})
        </span>
      </div>
      <div class="space-y-px max-h-36 overflow-y-auto">
        <button
          :for={a <- @archives}
          phx-click="view_archive"
          phx-value-room-id={a["room_id"]}
          class="w-full text-left px-2 py-1.5 rounded hover:bg-[var(--ch-hover-bg)] transition-colors flex items-center justify-between"
        >
          <span class="text-[11px] font-mono text-[var(--ch-text-mid)] truncate">{a["room_id"]}</span>
          <span class="text-[10px] text-[var(--ch-text-xs)] ml-2 shrink-0">
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
      <div class="bg-[var(--ch-float)] border border-[var(--ch-border)] rounded w-3/4 max-h-[80vh] overflow-hidden flex flex-col shadow-2xl">
        <div class="flex items-center justify-between px-4 py-2.5 border-b border-[var(--ch-border)] shrink-0">
          <span class="font-mono text-[12px] text-[var(--ch-text-mid)]">
            {@active_archive.room_id} / archive
          </span>
          <button
            phx-click="close_archive"
            class="text-[var(--ch-text-lo)] hover:text-[var(--ch-text-hi)] transition-colors"
            aria-label="Close archive"
          >
            <span class="hero-x-mark w-4 h-4"></span>
          </button>
        </div>
        <div class="overflow-y-auto px-5 py-4 council-prose text-[var(--ch-text-mid)]">
          {Phoenix.HTML.raw(CouncilHubUiWeb.CouncilHelpers.render_markdown(@active_archive.content))}
        </div>
      </div>
    </div>
    """
  end
end
