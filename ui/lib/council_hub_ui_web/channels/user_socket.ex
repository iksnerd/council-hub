defmodule CouncilHubUiWeb.UserSocket do
  @moduledoc """
  Socket for agent presence (`/socket`). Carries only the `presence:agents`
  channel; it cannot read or write messages.

  Origin policy: a request that carries an `Origin` header is refused. Non-browser
  clients (agent sidecars) send none and pass; a browser always sends one, so a
  web page can never open this socket from a visitor's browser. This is set on
  the socket itself because the release config sets `check_origin: false` for the
  whole endpoint (the dashboard's own LiveView socket needs that), and a socket
  inherits the endpoint setting unless it overrides it.

  Authentication: when `COUNCIL_AUTH_TOKEN` is set, `connect/3` requires the
  header `X-Council-Token: <token>`, compared in constant time. The token is
  never read from query parameters, which end up in logs. A connection that came
  in through the trusted listener (`COUNCIL_TRUSTED_PORT`) needs no token: it
  carries `x-council-trusted` with the per-boot secret, compared in constant time. With no token
  configured, any non-browser client that can reach the port can claim a
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
  def connect(_params, socket, connect_info) do
    if CouncilHubUi.Presence.enabled?() and authorized?(connect_info),
      do: {:ok, socket},
      else: :error
  end

  defp authorized?(connect_info) do
    if CouncilHubUi.Auth.enabled?() do
      headers = Map.get(connect_info, :x_headers, [])
      token_ok?(headers) or trusted?(headers)
    else
      true
    end
  end

  defp token_ok?(headers) do
    case List.keyfind(headers, "x-council-token", 0) do
      {_, value} -> CouncilHubUi.Auth.valid?(value)
      nil -> false
    end
  end

  # Only the trusted listener adds this header, with its per-boot secret
  # (CouncilHubUi.TrustedListener); every other entry deletes a client's copy.
  defp trusted?(headers) do
    case List.keyfind(headers, CouncilHubUi.TrustedListener.header(), 0) do
      {_, value} -> CouncilHubUi.TrustedListener.valid_marker?(value)
      nil -> false
    end
  end

  @impl true
  def id(_socket), do: nil
end
