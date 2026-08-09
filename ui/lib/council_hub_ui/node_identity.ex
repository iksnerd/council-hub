defmodule CouncilHubUi.NodeIdentity do
  @moduledoc """
  Detects drift between this node's registered distribution address
  (`RELEASE_NODE`, baked in once at container/release start) and the host's
  actual current network address, and can rebind live to correct it.

  DHCP can reassign a long-lived container's host IP without the container
  restarting — `entrypoint.sh` only re-detects the IP on `docker run`, not on
  every boot of a `--restart always` container — so a node can keep
  advertising a dead address indefinitely with no visible symptom (no crash,
  `/health` stays green, local MCP tools keep working) while being
  permanently unreachable to peers under that name.
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
  Compare the registered address against the current one. `drifted?` is only
  ever true when distributed and both addresses are known — an nil `current`
  (e.g. no route, sandboxed test env) never trips a false alarm.
  """
  def status do
    case registered_host() do
      nil ->
        %{registered: nil, current: nil, drifted?: false}

      registered ->
        current = current_ip()

        %{
          registered: registered,
          current: current,
          drifted?: present?(current) and registered != current
        }
    end
  end

  @doc """
  Rebind live distribution to `new_host`, preserving the node's short name
  and cookie. This drops all current peer connections — acceptable here,
  since under the stale name this node was already unreachable to anyone
  trying to connect in. The caller is responsible for re-triggering peer
  reconnects afterward (the new node identity has none by definition).
  """
  def rebind(new_host) when is_binary(new_host) do
    old = Node.self()
    cookie = Node.get_cookie()
    name = old |> to_string() |> String.split("@", parts: 2) |> hd()
    new_node = :"#{name}@#{new_host}"

    Logger.warning("NodeIdentity: rebinding distribution #{old} -> #{new_node}")

    with :ok <- :net_kernel.stop(),
         {:ok, _pid} <- Node.start(new_node, :longnames, 15_000) do
      Node.set_cookie(cookie)
      {:ok, new_node}
    else
      {:error, reason} = err ->
        Logger.error("NodeIdentity: rebind to #{new_node} failed: #{inspect(reason)}")
        err
    end
  end

  defp present?(nil), do: false
  defp present?(""), do: false
  defp present?(_), do: true
end
