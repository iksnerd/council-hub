defmodule CouncilHubUiWeb.PanelComponents do
  @moduledoc "Sidebar panel function components: mentions, archives, archive modal."

  use Phoenix.Component

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
