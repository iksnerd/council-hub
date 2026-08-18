defmodule CouncilHubUi.ClusterManager do
  @moduledoc """
  Runtime cluster membership management.

  Lets the dashboard connect/disconnect Erlang peer nodes live — no container
  restart — using `Node.connect/1` / `Node.disconnect/1`. Managed peers are
  persisted to a file in the data volume so they are re-connected on the next
  boot, complementing (not replacing) the libcluster `COUNCIL_SEEDS` strategy.

  This is the only write path in the otherwise read-only UI, and the settings
  page that drives it is gated to localhost.

  ## Self-heal

  Neither libcluster path re-forms a cluster after the first connection: the
  `Epmd` strategy dials its `hosts` once and never re-polls (its
  `polling_interval` is honored only by `Gossip`/`Kubernetes`), and this manager
  historically connected only at boot + on explicit UI action. So a peer that
  was down at boot, or a dist link that later dropped (laptop sleep, Wi-Fi blip,
  net-tick timeout), stayed disconnected until a container restart.

  To fix that this manager now:

    * subscribes to `:net_kernel.monitor_nodes/1`, so it learns about every peer
      that connects by *any* path (Gossip auto-discovery, Epmd seeds, or the UI)
      and remembers it in `known`;
    * on a `~10s` timer, re-dials any `known` peer that is not currently in
      `Node.list/0`.

  Explicit UI disconnect removes a peer from `known`, so the loop won't undo it.

  ## Own-identity drift

  The above only fixes a *peer* going stale — it assumes this node's own
  `RELEASE_NODE` is still valid. It isn't always: DHCP can reassign the host's
  LAN IP out from under a long-lived `--restart always` container without a
  restart (`entrypoint.sh`'s IP auto-detect only runs on `docker run`), which
  leaves this node advertising a dead address with zero visible symptoms.

  The same `~10s` tick also runs `CouncilHubUi.NodeIdentity.status/0`. On
  detected drift (rate-limited to one attempt per `@rebind_cooldown`) it calls
  `NodeIdentity.rebind/1` to restart distribution under the corrected
  address, then immediately re-dials `known` peers under the new identity —
  *when* this boot allows it.

  Two guards keep that from being noise. `NodeIdentity` only reports drift
  where the comparison is meaningful at all (see its moduledoc — under
  bridge networking the measured address is the container's, not the host's,
  and would otherwise read as permanent drift on a healthy node). And every
  normal boot uses static distribution (`RELEASE_DISTRIBUTION=name`), which
  `rebind/1` cannot tear down: `init/1` reads that once via
  `NodeIdentity.static_distribution?/0` and records
  `self_heal_supported?: false`, so this loop never attempts a call that can
  only ever fail — it logs the situation once, on the first drift it sees,
  and keeps reporting. The latest status (including `self_heal_supported?`)
  is cached in state and exposed via `ip_status/1` for `/health` and the
  `/status` config doctor.
  """
  use GenServer
  require Logger

  alias CouncilHubUi.NodeIdentity

  # name@host — host may be an IP, hostname, or Tailscale MagicDNS name.
  @node_re ~r/^[^@\s]+@[^@\s]+$/

  # How often to re-dial known-but-disconnected peers / re-check own identity.
  @reconnect_interval :timer.seconds(10)

  # Minimum gap between self-heal rebind attempts, so a persistent failure
  # (e.g. no route to the "current" IP either) doesn't retry every tick.
  @rebind_cooldown :timer.seconds(60)

  ## Client API

  def start_link(opts) do
    name = Keyword.get(opts, :name, __MODULE__)
    GenServer.start_link(__MODULE__, opts, name: name)
  end

  @doc "Connect to a peer node (string like `council_hub@100.x.y.z`) and persist it."
  def connect(server \\ __MODULE__, node_str) when is_binary(node_str) do
    GenServer.call(server, {:connect, String.trim(node_str)})
  end

  @doc "Disconnect a peer node and drop it from the persisted set."
  def disconnect(server \\ __MODULE__, node_str) when is_binary(node_str) do
    GenServer.call(server, {:disconnect, String.trim(node_str)})
  end

  @doc "List the node names this manager is responsible for (persisted), as strings."
  def managed_peers(server \\ __MODULE__) do
    GenServer.call(server, :managed_peers)
  end

  @doc "List every peer the self-heal loop keeps alive (persisted ∪ seeds ∪ seen), as strings."
  def known_peers(server \\ __MODULE__) do
    GenServer.call(server, :known_peers)
  end

  @doc """
  This node's own address-drift status, as last computed by the reconnect
  tick: `%{registered:, current:, drifted?:, checkable?:, self_heal_supported?:}`.
  `registered`/`current` are nil when not distributed; `checkable?` is false
  when the comparison isn't meaningful in this deployment (see
  `CouncilHubUi.NodeIdentity`), in which case `drifted?` is never true.
  """
  def ip_status(server \\ __MODULE__) do
    GenServer.call(server, :ip_status)
  end

  ## Server callbacks

  @impl true
  def init(opts) do
    path = Keyword.get(opts, :path) || default_path()
    interval = Keyword.get(opts, :reconnect_interval, @reconnect_interval)
    peers = read_peers(path)

    # Learn about peers that connect/drop by any path (Gossip, Epmd seeds, UI)
    # so the reconnect loop can re-dial them after a dropped dist link.
    monitor_nodes()

    # Seed the keep-alive set from persisted peers, `node@host` COUNCIL_SEEDS,
    # and anything already connected. Bare-IP seeds are left to libcluster/Go
    # discovery (they aren't valid node names to Node.connect/1).
    known =
      peers
      |> MapSet.union(read_seed_nodes())
      |> Enum.map(&String.to_atom/1)
      |> Enum.concat(Node.list())
      |> MapSet.new()

    # Establish connections on boot.
    for node <- known do
      case safe_connect(node) do
        true ->
          Logger.info("ClusterManager: connected to #{node}")

        other ->
          Logger.warning("ClusterManager: could not connect to #{node} (#{inspect(other)})")
      end
    end

    schedule_reconnect(interval)

    {:ok,
     %{
       path: path,
       peers: peers,
       known: known,
       interval: interval,
       ip_status: NodeIdentity.status(),
       last_rebind_attempt: nil,
       # Knowable at boot, so don't learn it by failing a rebind first.
       self_heal_supported?: not NodeIdentity.static_distribution?(),
       self_heal_warned?: false
     }}
  end

  @impl true
  def handle_call({:connect, node_str}, _from, state) do
    with :ok <- validate(node_str, state),
         node <- String.to_atom(node_str),
         true <- safe_connect(node) do
      peers = MapSet.put(state.peers, node_str)
      write_peers(state.path, peers)
      {:reply, :ok, %{state | peers: peers, known: MapSet.put(state.known, node)}}
    else
      {:error, reason} ->
        {:reply, {:error, reason}, state}

      false ->
        {:reply,
         {:error,
          "could not reach #{node_str} — check the IP, that it's running, and the cookie matches"},
         state}

      :ignored ->
        {:reply,
         {:error,
          "this node is not running in distributed mode (no RELEASE_NODE) — cannot connect peers"},
         state}
    end
  end

  @impl true
  def handle_call({:disconnect, node_str}, _from, state) do
    node = String.to_atom(node_str)
    Node.disconnect(node)
    peers = MapSet.delete(state.peers, node_str)
    write_peers(state.path, peers)
    # Drop from `known` too, so the reconnect loop doesn't immediately re-dial it.
    {:reply, :ok, %{state | peers: peers, known: MapSet.delete(state.known, node)}}
  end

  @impl true
  def handle_call(:managed_peers, _from, state) do
    {:reply, MapSet.to_list(state.peers), state}
  end

  @impl true
  def handle_call(:known_peers, _from, state) do
    {:reply, state.known |> MapSet.to_list() |> Enum.map(&to_string/1), state}
  end

  @impl true
  def handle_call(:ip_status, _from, state) do
    {:reply, Map.put(state.ip_status, :self_heal_supported?, state.self_heal_supported?), state}
  end

  @impl true
  def handle_info(:reconnect, state) do
    state = maybe_self_heal(state)

    for node <- state.known, node != Node.self(), node not in Node.list() do
      # Re-dial quietly: a peer that's simply down shouldn't spam the log every tick.
      safe_connect(node)
    end

    schedule_reconnect(state.interval)
    {:noreply, state}
  end

  @impl true
  def handle_info({:nodeup, node}, state) do
    Logger.info("ClusterManager: node up #{node}")
    {:noreply, %{state | known: MapSet.put(state.known, node)}}
  end

  @impl true
  def handle_info({:nodedown, node}, state) do
    Logger.warning("ClusterManager: node down #{node} — will attempt reconnect")
    {:noreply, state}
  end

  @impl true
  def handle_info(_msg, state), do: {:noreply, state}

  ## Helpers

  # Re-checks address drift every tick; attempts a self-heal rebind when
  # drifted and not cooling down from a prior attempt. Always returns state
  # with a fresh `ip_status`.
  defp maybe_self_heal(state) do
    status = NodeIdentity.status()

    cond do
      not status.drifted? ->
        %{state | ip_status: status}

      not state.self_heal_supported? ->
        # This boot can never rebind (static distribution, read at init). Keep
        # surfacing drift on /health and /status, but say so once rather than
        # attempting — and logging — a guaranteed failure every tick.
        warn_self_heal_unavailable(state, status)

      cooling_down?(state.last_rebind_attempt) ->
        %{state | ip_status: status}

      true ->
        Logger.error(
          "ClusterManager: node identity stale — registered as #{Node.self()}, " <>
            "host is now #{status.current}. Attempting self-heal rebind."
        )

        state = %{state | last_rebind_attempt: monotonic_ms()}

        case NodeIdentity.rebind(status.current) do
          {:ok, new_node} ->
            Logger.info("ClusterManager: rebound to #{new_node} — re-dialing known peers")

            for node <- state.known, node != new_node do
              safe_connect(node)
            end

            %{state | ip_status: NodeIdentity.status()}

          {:error, :static_distribution} ->
            # init/1 read this wrong (older OTP, no :net_kernel.get_state/0) —
            # correct it now so the next tick takes the branch above instead of
            # retrying a structurally impossible call.
            %{state | ip_status: status, self_heal_supported?: false, self_heal_warned?: true}

          {:error, _reason} ->
            # Logged inside NodeIdentity.rebind/1; keep the pre-heal status so
            # the doctor still shows "drifted" rather than silently clearing.
            %{state | ip_status: status}
        end
    end
  end

  # Report a drift this boot can't heal — once, not every tick.
  defp warn_self_heal_unavailable(%{self_heal_warned?: true} = state, status) do
    %{state | ip_status: status}
  end

  defp warn_self_heal_unavailable(state, status) do
    Logger.error(
      "ClusterManager: node identity stale — registered as #{Node.self()}, " <>
        "host is now #{status.current}. Self-heal is unavailable on this boot " <>
        "(distribution was started statically); restart the container to re-detect " <>
        "the address. This is logged once per boot."
    )

    %{state | ip_status: status, self_heal_warned?: true}
  end

  defp cooling_down?(nil), do: false
  defp cooling_down?(last_attempt_ms), do: monotonic_ms() - last_attempt_ms < @rebind_cooldown

  defp monotonic_ms, do: System.monotonic_time(:millisecond)

  defp schedule_reconnect(interval), do: Process.send_after(self(), :reconnect, interval)

  # Subscribe to peer up/down events; a no-op-safe call when net_kernel isn't up.
  defp monitor_nodes do
    :net_kernel.monitor_nodes(true)
  rescue
    _ -> :ok
  catch
    _, _ -> :ok
  end

  # `node@host` entries from COUNCIL_SEEDS (bare IPs/hostnames are filtered out —
  # they aren't connectable node names and are handled by libcluster/Go discovery).
  defp read_seed_nodes do
    System.get_env("COUNCIL_SEEDS", "")
    |> String.split(",", trim: true)
    |> Enum.map(&String.trim/1)
    |> Enum.filter(&Regex.match?(@node_re, &1))
    |> MapSet.new()
  end

  defp validate(node_str, state) do
    cond do
      not Regex.match?(@node_re, node_str) ->
        {:error, "invalid node name — expected something like council_hub@100.x.y.z"}

      node_str == to_string(Node.self()) ->
        {:error, "that's this node — pick a peer's node name"}

      MapSet.member?(state.peers, node_str) and
          Enum.member?(Node.list(), String.to_atom(node_str)) ->
        {:error, "already connected to #{node_str}"}

      true ->
        :ok
    end
  end

  # Node.connect returns true | false | :ignored (when not distributed).
  defp safe_connect(node) when is_atom(node), do: Node.connect(node)
  defp safe_connect(node) when is_binary(node), do: Node.connect(String.to_atom(node))

  defp read_peers(path) do
    case File.read(path) do
      {:ok, contents} ->
        contents
        |> String.split("\n", trim: true)
        |> Enum.map(&String.trim/1)
        |> Enum.reject(&(&1 == ""))
        |> MapSet.new()

      {:error, _} ->
        MapSet.new()
    end
  end

  defp write_peers(path, peers) do
    contents = peers |> MapSet.to_list() |> Enum.sort() |> Enum.join("\n")

    with :ok <- File.mkdir_p(Path.dirname(path)),
         :ok <- File.write(path, contents) do
      :ok
    else
      {:error, reason} ->
        Logger.warning("ClusterManager: could not persist peers to #{path}: #{inspect(reason)}")
        :ok
    end
  end

  @doc "Default peers file: alongside the SQLite DB, else a tmp fallback."
  def default_path do
    case System.get_env("COUNCIL_DB_PATH") do
      nil -> Path.join(System.tmp_dir!(), "council_hub_cluster_peers")
      db_path -> Path.join(Path.dirname(db_path), "cluster_peers")
    end
  end
end
