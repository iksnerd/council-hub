defmodule CouncilHubUi.NodeIdentity do
  @moduledoc """
  Detects drift between this node's registered distribution address
  (`RELEASE_NODE`, baked in once at container/release start) and the host's
  actual current network address, and can rebind live to correct it *when
  the deployment makes that check meaningful and the boot mode allows it*.

  DHCP can reassign a long-lived container's host IP without the container
  restarting — `entrypoint.sh` only re-detects the IP on `docker run`, not on
  every boot of a `--restart always` container — so a node can keep
  advertising a dead address indefinitely with no visible symptom (no crash,
  `/health` stays green, local MCP tools keep working) while being
  permanently unreachable to peers under that name.

  ## When the comparison is valid at all

  `current_ip/0` measures the address of *the network namespace this process
  runs in*. Under Docker's default bridge networking that is the container's
  address (`172.x.y.z`), while `RELEASE_NODE` is deliberately set to the
  *host's* LAN IP so peers can reach the published port. Those two never
  match, so a naive comparison reports permanent drift on a perfectly healthy
  clustered node — and a rebind onto the measured address would move a
  working node to a container-internal IP that no peer can reach.

  The same applies to a `RELEASE_NODE` pointing at a tailnet address
  (`alice@100.x.y.z` — the route to the probe target leaves via the physical
  interface, not the tailnet) or at a hostname/MagicDNS name, which never
  string-equals an IP.

  So the check runs only when the registered name and the measured address
  come from the same place:

    * `COUNCIL_NODE_AUTODETECTED=1` — exported by `entrypoint.sh` when *it*
      derived `RELEASE_NODE` from this namespace's default route. The
      comparison is then apples-to-apples by construction.
    * `COUNCIL_NODE_DRIFT_CHECK=1` — operator opt-in, for an explicitly-set
      `RELEASE_NODE` on a container that shares the host's network namespace
      (`--network host`). `COUNCIL_NODE_DRIFT_CHECK=0` force-disables.

  Otherwise `status/0` reports `checkable?: false` and never claims drift.

  ## When a live rebind is possible

  Only when distribution was started dynamically. Every normal boot of this
  image instead starts it statically, via `RELEASE_DISTRIBUTION=name` (the
  `mix release` default) driving `erl -name`, which OTP will not let
  `:net_kernel.stop/0` tear down — so `rebind/1` always fails with
  `{:error, :static_distribution}` there. Drift detection is still useful in
  that mode (`status/0` surfaces it on `/health` and `/status`), but the fix
  is a container restart, which re-runs the entrypoint's detection.
  """
  require Logger

  alias CouncilHubUiWeb.CouncilHelpers

  @doc "The host portion of this node's registered distribution name, or nil if not distributed."
  def registered_host do
    case to_string(Node.self()) do
      "nonode@nohost" -> nil
      node -> CouncilHelpers.node_host(node)
    end
  end

  @doc """
  The IP this host would currently use to reach the LAN — found the same way
  `ip route get 1` works at the shell level: open a UDP socket "connected" to
  an arbitrary routable address and read back the local address the kernel
  picked for that route. No packet is actually sent (UDP `connect/3` only
  consults the local routing table), so this works even without real
  internet access, and needs no new dependency (`:gen_udp`/`:inet` are OTP
  stdlib).

  This is the address of *this process's* network namespace — under bridge
  networking that is the container, not the Docker host. See the moduledoc.
  """
  def current_ip do
    case :gen_udp.open(0, active: false) do
      {:ok, socket} ->
        try do
          with :ok <- :gen_udp.connect(socket, {8, 8, 8, 8}, 53),
               {:ok, {ip, _port}} <- :inet.sockname(socket) do
            ip |> :inet.ntoa() |> to_string()
          else
            _ -> nil
          end
        after
          :gen_udp.close(socket)
        end

      {:error, _reason} ->
        nil
    end
  end

  @doc """
  Whether comparing the registered address against the measured one says
  anything about reachability in this deployment. See the moduledoc — false
  unless the entrypoint derived the node name from this namespace, or the
  operator opted in.
  """
  def drift_check_enabled? do
    case System.get_env("COUNCIL_NODE_DRIFT_CHECK") do
      nil -> truthy?(System.get_env("COUNCIL_NODE_AUTODETECTED"))
      forced -> truthy?(forced)
    end
  end

  @doc """
  Compare the registered address against the current one, reading both from
  the live node. Delegates to `status/3`.
  """
  def status do
    case registered_host() do
      nil -> status(nil, nil, false)
      registered -> status(registered, current_ip(), drift_check_enabled?())
    end
  end

  @doc """
  Pure form: decide drift from the three inputs, with no VM or environment
  reads, so the logic is testable without a distributed node.

  `drifted?` is true only when the comparison is meaningful (`check_enabled?`),
  this node is distributed, and both addresses are known and differ. A nil
  `current` (no route, sandboxed test env) never trips a false alarm, and
  `checkable?` records whether the comparison was made at all.
  """
  def status(registered, current, check_enabled?) do
    checkable? = present?(registered) and present?(current) and check_enabled?

    %{
      registered: registered,
      current: current,
      drifted?: checkable? and registered != current,
      checkable?: checkable?
    }
  end

  @doc """
  Rebind live distribution to `new_host`, preserving the node's short name
  and cookie. This drops all current peer connections — acceptable here,
  since under the stale name this node was already unreachable to anyone
  trying to connect in. The caller is responsible for re-triggering peer
  reconnects afterward (the new node identity has none by definition).

  Structurally impossible — not merely likely to fail — when this release
  booted distribution via a boot-time flag (`RELEASE_DISTRIBUTION=name`,
  the `mix release` default; also `sname`), which is every non-dev boot of
  this image. OTP's `:net_kernel.stop/0` refuses to tear down distribution
  that wasn't started dynamically via `Node.start/2`, returning
  `{:error, :not_allowed}` — no retry or timing fixes that. Callers should
  check `static_distribution?/0` (or just read this function's `{:error,
  :static_distribution}` return) and stop retrying rather than treat it as
  a transient failure.

  > #### Do not enable this by switching to dynamic distribution {: .warning}
  >
  > Making `RELEASE_DISTRIBUTION=none` + `Node.start/2` the boot path would
  > make this function succeed — including on a node whose "drift" is the
  > bridge-networking false positive described in the moduledoc, where it
  > would move a healthy node onto a container-internal address no peer can
  > reach. Only ever rebind onto an address measured from the same namespace
  > that produced `RELEASE_NODE` (what `drift_check_enabled?/0` guarantees).
  """
  def rebind(new_host) when is_binary(new_host) do
    if static_distribution?() do
      Logger.warning(
        "NodeIdentity: cannot rebind #{Node.self()} — distribution was started " <>
          "statically (RELEASE_DISTRIBUTION=name/sname), which OTP does not allow " <>
          "tearing down at runtime. Restart the container to re-detect the address."
      )

      {:error, :static_distribution}
    else
      do_rebind(new_host)
    end
  end

  defp do_rebind(new_host) do
    old = Node.self()
    cookie = Node.get_cookie()
    name = old |> to_string() |> String.split("@", parts: 2) |> hd()
    new_node = :"#{name}@#{new_host}"

    Logger.warning("NodeIdentity: rebinding distribution #{old} -> #{new_node}")

    case :net_kernel.stop() do
      :ok ->
        case Node.start(new_node, :longnames, 15_000) do
          {:ok, _pid} ->
            Node.set_cookie(cookie)
            {:ok, new_node}

          {:error, reason} ->
            Logger.error("NodeIdentity: rebind to #{new_node} failed: #{inspect(reason)}")
            {:error, reason}
        end

      # Distribution turned out to be static after all (e.g. `get_state/0`
      # unavailable, so `static_distribution?/0` couldn't tell). Report it as
      # such so the caller stops retrying a call that can only ever fail.
      {:error, :not_allowed} ->
        Logger.warning(
          "NodeIdentity: cannot rebind #{old} — :net_kernel.stop/0 returned " <>
            ":not_allowed, i.e. distribution was started statically. " <>
            "Restart the container to re-detect the address."
        )

        {:error, :static_distribution}

      {:error, reason} ->
        Logger.error("NodeIdentity: rebind to #{new_node} failed: #{inspect(reason)}")
        {:error, reason}
    end
  end

  @doc """
  True when this node's distribution was started via a boot-time flag
  (`-name`/`-sname`, i.e. `RELEASE_DISTRIBUTION` set for the release) rather
  than dynamically via `Node.start/2`. Only nodes started dynamically can be
  torn down and rebound live with `:net_kernel.stop/0`.

  Fails safe: if the state can't be read (`:net_kernel.get_state/0` is
  OTP 25+), assume static, so a caller skips a rebind it can't verify is
  possible rather than retrying a guaranteed failure forever.
  """
  def static_distribution? do
    case :net_kernel.get_state() do
      %{started: :dynamic} -> false
      # Not distributed at all — nothing to tear down, so nothing is blocked.
      %{started: :no} -> false
      _ -> true
    end
  rescue
    _ -> true
  catch
    _, _ -> true
  end

  defp truthy?(nil), do: false

  defp truthy?(value) when is_binary(value) do
    normalized = value |> String.trim() |> String.downcase()
    normalized in ["1", "true", "yes", "on"]
  end

  defp present?(nil), do: false
  defp present?(""), do: false
  defp present?(_), do: true

  @probe_timeout 2_000

  @doc """
  Whether anything answers on the address this node advertises to peers.

  `status/0` above compares two addresses and, in the deployment the docs
  recommend, refuses to compare at all (`checkable?: false`, see the moduledoc)
  because a bridged container cannot observe its host's address. This asks a
  different question that needs no such knowledge: *open a connection to the
  address we are telling peers to use.* Under bridge networking that leaves the
  container, reaches the host's published port and comes back — exactly the path
  a peer takes, which is exactly what needs testing. When a DHCP lease moves the
  host, that connection fails while everything else about the node stays green.

  Detection only: a failure here is reported, never acted on. The remedy is a
  container restart (which re-runs the entrypoint's detection) or a corrected
  `RELEASE_NODE`, neither of which this process can do safely on its own.
  """
  def advertised_status, do: advertised_status(registered_host(), epmd_port(), &probe/2)

  @doc "Pure form: the three inputs, no VM or environment reads. See `advertised_status/0`."
  def advertised_status(host, port, probe)

  def advertised_status(nil, _port, _probe), do: unknown_advertisement(nil)

  def advertised_status(host, port, probe) do
    if loopback?(host) do
      # A loopback RELEASE_NODE is unreachable to peers by construction, and
      # the /status doctor already says so. Probing it would succeed and imply
      # the opposite.
      unknown_advertisement(host)
    else
      case probe.(host, port) do
        :ok ->
          %{host: host, port: port, checkable?: true, reachable?: true, warning: nil}

        {:error, reason} ->
          %{
            host: host,
            port: port,
            checkable?: true,
            reachable?: false,
            warning:
              "nothing answers on #{host}:#{port}, the address this node advertises to peers " <>
                "(#{inspect(reason)}). Peers cannot reach it under that name. The host's IP " <>
                "has probably changed, or the cluster ports are published on a different " <>
                "address. Restart the container to re-detect it, or correct RELEASE_NODE."
          }
      end
    end
  end

  @doc "The EPMD port peers would use to reach this node."
  def epmd_port do
    case System.get_env("ERL_EPMD_PORT") do
      nil -> 4369
      "" -> 4369
      port -> String.to_integer(port)
    end
  end

  defp unknown_advertisement(host) do
    %{host: host, port: nil, checkable?: false, reachable?: nil, warning: nil}
  end

  defp loopback?(host) do
    String.downcase(host) in ["127.0.0.1", "localhost", "::1", "[::1]"]
  end

  defp probe(host, port) do
    case :gen_tcp.connect(
           String.to_charlist(host),
           port,
           [:binary, active: false],
           @probe_timeout
         ) do
      {:ok, socket} ->
        :gen_tcp.close(socket)
        :ok

      {:error, reason} ->
        {:error, reason}
    end
  end
end
