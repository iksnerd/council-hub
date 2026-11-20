defmodule CouncilHubUi.ClusterCookie do
  @moduledoc """
  Whether this BEAM runs the published default cookie (`council`).

  The answer comes from `Node.get_cookie/0`, the cookie the VM actually uses,
  not from an environment variable that may or may not have been applied.
  Only the boolean ever leaves this module; the cookie value is never returned
  by `default?/0`, logged or serialised.
  """

  @default :council

  @doc "True when this node's effective cookie is the published default."
  def default?, do: default?(current())

  @doc false
  def default?(cookie), do: cookie == @default

  # Overridable in tests: a non-distributed test VM has no cookie to read.
  defp current do
    reader = Application.get_env(:council_hub_ui, :cookie_reader, &Node.get_cookie/0)
    reader.()
  end

  @doc """
  The `/status` warning, or nil. Only when the default cookie is in use AND
  clustering is: a single node on the default cookie exposes nothing to a peer.
  """
  def warning(default?, seeds, peers) do
    clustered? = seeds not in [nil, ""] or peers != []

    if default? and clustered? do
      "This cluster uses the default cookie, which is public. Set RELEASE_COOKIE to the " <>
        "same non-default value on every node (a non-default cookie also lets the cluster " <>
        "ports bind to all interfaces)."
    end
  end
end
