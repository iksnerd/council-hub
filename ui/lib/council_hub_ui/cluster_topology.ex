defmodule CouncilHubUi.ClusterTopology do
  @moduledoc """
  Builds the libcluster topology from `COUNCIL_SEEDS`, for `config/runtime.exs`.

  ## Why gossip runs alongside Epmd rather than instead of it

  Seeds name peers by address, and on a DHCP network an address is a lease,
  not an identity. When a lease moves, every seed pointing at the old address
  is wrong, `entrypoint.sh`'s resolution silently launders the stale name, and
  the cluster stays split until someone notices — which, once, took four days.

  Gossip (UDP multicast, no addresses configured anywhere) makes that class of
  failure a non-event: peers find each other again wherever they moved to. It
  used to run only as the *fallback* for a node with no seeds, i.e. never in
  the deployment the docs recommend. It now runs next to Epmd, so seeds stay
  authoritative for the first connection and gossip covers the case where they
  have gone stale.

  It is not a replacement. Multicast does not cross a Docker bridge to the LAN,
  so gossip only reaches peers when the container shares the host's network
  (`--network host`) or the network otherwise passes multicast. Where it cannot,
  it is inert rather than harmful. `COUNCIL_GOSSIP=0` turns it off.
  """

  @gossip_port 45_892

  @doc """
  The topology keyword list. `seeds` is `COUNCIL_SEEDS`; `gossip` is
  `COUNCIL_GOSSIP` (nil means the default, which is on).
  """
  def build(seeds, gossip) do
    epmd_topology(seeds) ++ gossip_topology(gossip)
  end

  defp epmd_topology(seeds) when seeds in [nil, ""], do: []

  defp epmd_topology(seeds) do
    hosts =
      seeds
      |> String.split(",", trim: true)
      |> Enum.map(&String.to_atom(String.trim(&1)))

    # NOTE: Epmd connects to `hosts` once at boot and never re-polls
    # (`polling_interval` is honored only by Gossip/Kubernetes, not Epmd), so
    # it is omitted here to avoid implying otherwise. CouncilHubUi.ClusterManager
    # runs the periodic reconnect that actually self-heals a dropped link.
    [council_hub: [strategy: Cluster.Strategy.Epmd, config: [hosts: hosts]]]
  end

  defp gossip_topology(gossip) do
    if gossip_enabled?(gossip) do
      [
        council_hub_gossip: [
          strategy: Cluster.Strategy.Gossip,
          config: [polling_interval: 3_000, port: @gossip_port]
        ]
      ]
    else
      []
    end
  end

  defp gossip_enabled?(nil), do: true

  defp gossip_enabled?(value) do
    String.downcase(String.trim(value)) not in ["0", "false", "off", "no"]
  end
end
