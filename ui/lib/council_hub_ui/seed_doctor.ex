defmodule CouncilHubUi.SeedDoctor do
  @moduledoc """
  Detects the failure a node cannot see in its own mirror: a *peer* advertising
  a distribution name whose host address it no longer holds.
  """

  alias CouncilHubUiWeb.CouncilHelpers

  @probe_timeout 2_000

  @doc "Classify each configured seed, probing over HTTP."
  def check(seeds, peers), do: check(seeds, peers, &probe/1)

  @doc """
  Classify each configured seed. Returns [] when there is nothing to diagnose.
  """
  def check(seeds, peers, probe) when is_function(probe, 1) do
    # A cluster with any peer connected is not the failure this looks for, and
    # a healthy node must not issue a probe every tick for the rest of its life.
    if peers == [] do
      seeds
      |> seed_hosts()
      |> Enum.map(&classify(&1, probe))
    else
      []
    end
  end

  defp seed_hosts(seeds) do
    seeds
    |> to_string()
    |> String.split(",", trim: true)
    |> Enum.map(&String.trim/1)
    |> Enum.reject(&(&1 == "" or loopback?(&1)))
  end

  # A seed reached over loopback tells us nothing about addresses: the node
  # answering there legitimately advertises its LAN IP, so the comparison this
  # module exists to make is not meaningful. Same guard as NodeIdentity's
  # `checkable?` — silence beats a verdict that cannot be trusted.
  defp loopback?(seed) do
    host = String.downcase(CouncilHelpers.node_host(seed))
    host in ["127.0.0.1", "localhost", "::1", "[::1]"]
  end

  defp classify(seed, probe) do
    host = CouncilHelpers.node_host(seed)

    case probe.(host) do
      {:ok, reported} ->
        if CouncilHelpers.node_host(reported) == host do
          finding(seed, host, reported, :undialable)
        else
          finding(seed, host, reported, :stale_name)
        end

      {:error, :no_node_in_health} ->
        finding(seed, host, nil, :no_node_name)

      _ ->
        finding(seed, host, nil, :unreachable)
    end
  end

  defp finding(seed, host, reported, status) do
    %{
      seed: seed,
      host: host,
      reported_node: reported,
      status: status,
      message: message(host, reported, status)
    }
  end

  defp message(host, reported, :stale_name) do
    "seed #{host} is up but reports node #{reported}, an address it does not hold. " <>
      "That node is undialable under any name until it is restarted with a RELEASE_NODE " <>
      "matching #{host}"
  end

  defp message(host, reported, :undialable) do
    "seed #{host} is up and reports #{reported}, but no link formed. Check that " <>
      "RELEASE_COOKIE matches on both nodes and that 4369/9000 are published and reachable"
  end

  defp message(host, _reported, :no_node_name) do
    "seed #{host} answered but reported no node name. It may be running with " <>
      "COUNCIL_UI=off, in which case its identity cannot be checked from here"
  end

  defp message(host, _reported, :unreachable) do
    "seed #{host} is not answering. The peer is down, or its address changed"
  end

  @doc """
  Ask a seed host what node name it believes it has: the Go server's `/health`
  reports it, and EPMD alone cannot (it only knows short names).
  """
  def probe(host) do
    url = host |> probe_url() |> String.to_charlist()

    case :httpc.request(:get, {url, []}, [{:timeout, @probe_timeout}], []) do
      {:ok, {{_http, status, _}, _headers, body}} when status in 200..299 ->
        parse_node(body)

      {:ok, {{_http, status, _}, _, _}} ->
        {:error, {:http_error, status}}

      {:error, reason} ->
        {:error, reason}
    end
  rescue
    e -> {:error, e}
  end

  @doc "The /health URL used to probe a seed host. Port from COUNCIL_PEER_MCP_PORT."
  def probe_url(host) do
    port = System.get_env("COUNCIL_PEER_MCP_PORT") || "3001"
    "http://#{host}:#{port}/health"
  end

  @doc """
  Collapse findings into one line for `/health`, stale names first — that is
  the half of the failure the node reporting it cannot fix or even observe
  about itself.
  """
  def warning([]), do: nil

  def warning(findings) do
    detail =
      findings
      |> Enum.sort_by(&order(&1.status))
      |> Enum.map_join("; ", & &1.message)

    "no cluster peers connected. #{detail}"
  end

  defp order(:stale_name), do: 0
  defp order(:undialable), do: 1
  defp order(:no_node_name), do: 2
  defp order(_), do: 3

  defp parse_node(body) do
    case Jason.decode(to_string(body)) do
      {:ok, %{"cluster_nodes" => [%{"node" => node} | _]}} when is_binary(node) -> {:ok, node}
      {:ok, _} -> {:error, :no_node_in_health}
      {:error, _} -> {:error, :json_decode_failed}
    end
  end
end
