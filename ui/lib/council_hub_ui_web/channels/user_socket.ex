defmodule CouncilHubUiWeb.UserSocket do
  @moduledoc """
  Socket for agent presence (`/socket`). Carries only the `presence:agents`
  channel; it cannot read or write messages.

  Origin policy: a request that carries an `Origin` header is refused. Non-browser
  clients (agent sidecars) send none and pass; a browser always sends one, so a
  web page can never open this socket from a visitor's browser. This is set on
  the socket itself because the release config sets `check_origin: false` for the
  whole endpoint (the dashboard's own LiveView socket needs that), and a socket
  inherits the endpoint setting unless it overrides it. Nothing here is
  authenticated: any non-browser client that can reach the port can claim a
  presence entry.
  """

  use Phoenix.Socket

  @doc """
  Origin check for `/socket` (called by Phoenix with the parsed `Origin` URI,
  only when the header is present): always refuse.
  """
  @spec allow_origin?(URI.t()) :: false
  def allow_origin?(_origin), do: false

  channel "presence:agents", CouncilHubUiWeb.PresenceChannel

  @impl true
  def connect(_params, socket, _connect_info) do
    if CouncilHubUi.Presence.enabled?(), do: {:ok, socket}, else: :error
  end

  @impl true
  def id(_socket), do: nil
end
