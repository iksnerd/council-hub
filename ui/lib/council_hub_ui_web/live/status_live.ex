defmodule CouncilHubUiWeb.StatusLive do
  @moduledoc """
  Read-only health & status dashboard: node identity, cluster peers, DB stats,
  semantic-search coverage, and a config doctor that flags common misconfig.

  Public (no admin token) — it only reads. The live write surface (connecting
  peers) stays gated at `/settings`.
  """
  use CouncilHubUiWeb, :live_view
  require Logger

  alias CouncilHubUi.{ClusterManager, HealthStats, McpClient}
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

  @impl true
  def handle_event("regenerate_embeddings", params, socket) do
    full = Map.get(params, "full") == "true"

    case McpClient.backfill_embeddings(full) do
      {:ok, %{"started" => true} = result} ->
        mode = if full, do: "full re-embed", else: "backfill"

        msg =
          "Started #{mode} — #{result["msg_indexed"]}/#{result["msg_total"]} messages, " <>
            "#{result["room_indexed"]}/#{result["room_total"]} rooms indexed so far."

        {:noreply, put_flash(socket, :info, msg)}

      {:ok, %{"started" => false}} ->
        {:noreply,
         put_flash(socket, :error, "A backfill/re-embed is already running — try again shortly.")}

      {:error, reason} ->
        Logger.warning("regenerate_embeddings failed: #{inspect(reason)}")

        {:noreply,
         put_flash(
           socket,
           :error,
           "Regenerate failed — is COUNCIL_OLLAMA_URL set and the MCP server running in HTTP mode?"
         )}
    end
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
    ip_status = ClusterManager.ip_status()
    seed_findings = ClusterManager.seed_status().findings
    advertised = ClusterManager.advertised_status()

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
    |> assign(:ip_status, ip_status)
    |> assign(:stats, HealthStats.db_stats())
    |> assign(:short_node_fun, &short_node/1)
    |> assign(
      :warnings,
      doctor(
        self_node,
        distributed?,
        cookie_set?,
        seeds,
        peers,
        ip_status,
        seed_findings,
        advertised
      )
    )
  end

  defp version do
    case Application.spec(:council_hub_ui, :vsn) do
      nil -> "—"
      v -> to_string(v)
    end
  end

  # A small "config doctor": surfaces the foot-guns that otherwise only show up
  # as silent cluster failures.
  defp doctor(
         self_node,
         distributed?,
         cookie_set?,
         seeds,
         peers,
         ip_status,
         seed_findings,
         advertised
       ) do
    []
    |> maybe(not distributed?, "Not distributed — set RELEASE_NODE so peers can reach this node.")
    |> maybe(distributed? and not cookie_set?, "RELEASE_COOKIE not set — clustering is disabled.")
    |> maybe(
      distributed? and loopback?(self_node),
      "RELEASE_NODE points at loopback — cluster peers can't reach this node."
    )
    |> maybe(
      present?(seeds) and peers == [] and seed_findings == [],
      "Seeds are configured but no peers are connected yet — check the cookie matches and ports are reachable."
    )
    # Once the seeds have actually been probed, say what they reported rather
    # than the generic line: a peer advertising a dead address is a different
    # problem, with a different fix, on a different machine.
    |> then(fn warnings ->
      Enum.reduce(seed_findings, warnings, fn finding, acc ->
        [as_sentence(finding.message) | acc]
      end)
    end)
    |> maybe(
      ip_status.drifted? and ip_status.self_heal_supported?,
      "Node identity stale: registered as #{self_node}, host is now #{ip_status.current} — " <>
        "a self-heal rebind is retried automatically (~60s cooldown)."
    )
    |> maybe(
      ip_status.drifted? and not ip_status.self_heal_supported?,
      "Node identity stale: registered as #{self_node}, host is now #{ip_status.current} — " <>
        "self-heal is unavailable on this boot (distribution was started statically). " <>
        "Restart the container to re-detect the address, or set " <>
        "RELEASE_NODE/COUNCIL_NODE_NAME to the correct one."
    )
    |> maybe(advertised.warning != nil, as_sentence(advertised.warning || ""))
    |> Enum.reverse()
  end

  # Upcase the first letter only — String.capitalize/1 would downcase the rest,
  # turning RELEASE_COOKIE into release_cookie in the remedy it is offering.
  defp as_sentence(message) do
    {first, rest} = String.split_at(message, 1)
    sentence = String.upcase(first) <> rest
    if String.ends_with?(sentence, [".", "!", "?"]), do: sentence, else: sentence <> "."
  end

  defp maybe(list, true, msg), do: [msg | list]
  defp maybe(list, false, _msg), do: list

  defp loopback?(node), do: String.contains?(node, ["@127.0.0.1", "@localhost"])

  defp present?(nil), do: false
  defp present?(""), do: false
  defp present?(_), do: true
end
