defmodule CouncilHubUiWeb.StatusLive do
  @moduledoc """
  Read-only health & status dashboard: node identity, cluster peers, DB stats,
  semantic-search coverage, and a config doctor that flags common misconfig.

  Public (no admin token) — it only reads. The live write surface (connecting
  peers) stays gated at `/settings`.
  """
  use CouncilHubUiWeb, :live_view

  alias CouncilHubUi.HealthStats
  import CouncilHubUiWeb.CouncilHelpers, only: [short_node: 1]

  @refresh_interval 5_000

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket), do: Process.send_after(self(), :refresh, @refresh_interval)
    {:ok, assign_state(socket)}
  end

  @impl true
  def handle_info(:refresh, socket) do
    Process.send_after(self(), :refresh, @refresh_interval)
    {:noreply, assign_state(socket)}
  end

  ## Components

  attr :label, :string, required: true
  attr :value, :any, required: true

  defp stat(assigns) do
    ~H"""
    <div>
      <div class="text-[10px] text-[var(--ch-text-xs)] uppercase tracking-wide">{@label}</div>
      <div class="font-mono text-[18px] text-[var(--ch-text-hi)]">{@value}</div>
    </div>
    """
  end

  def present_seeds?(seeds), do: present?(seeds)

  ## Helpers

  defp assign_state(socket) do
    self_node = to_string(Node.self())
    distributed? = self_node != "nonode@nohost"
    cookie_set? = present?(System.get_env("RELEASE_COOKIE"))
    seeds = System.get_env("COUNCIL_SEEDS")
    peers = Node.list() |> Enum.map(&to_string/1) |> Enum.sort()

    socket
    |> assign(:page_title, "Status")
    |> assign(:self_node, self_node)
    |> assign(:distributed?, distributed?)
    |> assign(:cookie_set?, cookie_set?)
    |> assign(:transport, System.get_env("COUNCIL_TRANSPORT") || "http")
    |> assign(:version, version())
    |> assign(:db_path, System.get_env("COUNCIL_DB_PATH") || "—")
    |> assign(:seeds, seeds)
    |> assign(:peers, peers)
    |> assign(:stats, HealthStats.db_stats())
    |> assign(:short_node_fun, &short_node/1)
    |> assign(:warnings, doctor(self_node, distributed?, cookie_set?, seeds, peers))
  end

  defp version do
    case Application.spec(:council_hub_ui, :vsn) do
      nil -> "—"
      v -> to_string(v)
    end
  end

  # A small "config doctor": surfaces the foot-guns that otherwise only show up
  # as silent cluster failures.
  defp doctor(self_node, distributed?, cookie_set?, seeds, peers) do
    []
    |> maybe(not distributed?, "Not distributed — set RELEASE_NODE so peers can reach this node.")
    |> maybe(distributed? and not cookie_set?, "RELEASE_COOKIE not set — clustering is disabled.")
    |> maybe(
      distributed? and loopback?(self_node),
      "RELEASE_NODE points at loopback — cluster peers can't reach this node."
    )
    |> maybe(
      present?(seeds) and peers == [],
      "Seeds are configured but no peers are connected yet — check the cookie matches and ports are reachable."
    )
    |> Enum.reverse()
  end

  defp maybe(list, true, msg), do: [msg | list]
  defp maybe(list, false, _msg), do: list

  defp loopback?(node), do: String.contains?(node, ["@127.0.0.1", "@localhost"])

  defp present?(nil), do: false
  defp present?(""), do: false
  defp present?(_), do: true
end
