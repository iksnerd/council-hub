defmodule CouncilHubUi.TrustedListener do
  @moduledoc """
  The optional second listener behind `COUNCIL_TRUSTED_PORT`.

  Behind Docker Desktop every published-port request reaches the app from the
  bridge gateway, so the app cannot tell a local browser from a remote one by
  IP. The operator instead publishes two ports: the normal one (token
  enforced) on the tailnet or LAN address, and this one on host loopback only.
  A request is trusted only because it arrived through this listener, never
  because of anything the client sent.

  The marker is set by `CouncilHubUiWeb.Plugs.TrustedEntry`, the `plug:` of the
  second Bandit listener. `conn.private[:council_trusted]` covers the HTTP
  routes. WebSocket upgrades are checked in `UserSocket.connect/3`, which only
  sees `connect_info`, so the entry plug also adds an `x-council-trusted`
  request header carrying a random per-boot secret (`secret/0`). The endpoint
  deletes any client-supplied copy of that header on the main listener
  (`CouncilHubUiWeb.TrustedScrub`), and the entry plug deletes it before adding
  its own, so a client cannot present the marker on either listener.

  The listener binds all interfaces inside the container: the container
  network is the boundary, and the operator publishes the port on loopback.
  """

  @header "x-council-trusted"
  @secret_key {__MODULE__, :secret}

  @doc "Name of the marker request header."
  def header, do: @header

  @doc """
  Validates the raw `COUNCIL_TRUSTED_PORT` against the main port.

  `{:ok, nil}` when unset or empty, `{:ok, port}` when usable, else
  `{:error, message}`.
  """
  @spec validate(String.t() | nil, integer()) :: {:ok, nil | pos_integer()} | {:error, String.t()}
  def validate(raw, main_port) do
    case raw |> to_string() |> String.trim() do
      "" ->
        {:ok, nil}

      text ->
        case Integer.parse(text) do
          {port, ""} when port in 1..65_535 and port == main_port ->
            {:error,
             "COUNCIL_TRUSTED_PORT (#{port}) must differ from PORT (#{main_port}): " <>
               "the trusted listener skips the token, the main one enforces it."}

          {port, ""} when port in 1..65_535 ->
            {:ok, port}

          _ ->
            {:error, "COUNCIL_TRUSTED_PORT must be an integer from 1 to 65535."}
        end
    end
  end

  @doc "The configured trusted port, or nil when there is no trusted listener."
  def port do
    case Application.get_env(:council_hub_ui, :trusted_port) do
      p when is_integer(p) -> p
      _ -> nil
    end
  end

  @doc "True when the trusted listener is configured."
  def enabled?, do: port() != nil

  @doc "The `/status` line about the trusted listener."
  def status_line do
    case port() do
      nil -> "Trusted local listener: off"
      p -> "Trusted local listener: on (port #{p}; publish it on loopback only)"
    end
  end

  @doc "The per-boot secret, created on first use and kept in `:persistent_term`."
  @spec secret() :: String.t()
  def secret do
    case :persistent_term.get(@secret_key, nil) do
      nil ->
        value = Base.encode16(:crypto.strong_rand_bytes(32), case: :lower)
        :persistent_term.put(@secret_key, value)
        value

      value ->
        value
    end
  end

  @doc "Constant-time check of a header value against the per-boot secret."
  @spec valid_marker?(term()) :: boolean()
  def valid_marker?(value) when is_binary(value),
    do: Plug.Crypto.secure_compare(value, secret())

  def valid_marker?(_), do: false

  @doc """
  Child spec for the second Bandit listener. `opts` may override `:port`,
  `:ip` and `:plug` (tests start it on port 0).
  """
  def child_spec(opts) do
    bandit_opts =
      Keyword.merge(
        [
          plug: CouncilHubUiWeb.Plugs.TrustedEntry,
          scheme: :http,
          ip: {0, 0, 0, 0, 0, 0, 0, 0},
          port: port()
        ],
        opts
      )

    Supervisor.child_spec({Bandit, bandit_opts}, id: __MODULE__)
  end
end
