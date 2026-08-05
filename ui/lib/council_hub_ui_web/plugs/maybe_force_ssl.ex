defmodule CouncilHubUiWeb.Plugs.MaybeForceSsl do
  @moduledoc """
  Runtime opt-in HTTPS redirect (COUNCIL_FORCE_SSL=true).

  Phoenix's built-in `:force_ssl` endpoint option is read at compile time
  (`Application.compile_env` inside `Phoenix.Endpoint.__using__`), so setting it
  from config/runtime.exs is a silent no-op — this plug does the same job
  (delegating to `Plug.SSL`) but consults `:force_ssl_enabled` on each request.

  Loopback hosts are always excluded: the Go MCP server calls the internal
  cluster API at http://127.0.0.1:4000 and the container healthcheck probes
  localhost — neither must ever be redirected to an https:// this process does
  not serve.
  """

  @behaviour Plug

  @ssl_opts Plug.SSL.init(
              rewrite_on: [:x_forwarded_proto],
              exclude: [conn: {__MODULE__, :excluded?, []}]
            )

  @impl true
  def init(opts), do: opts

  @impl true
  def call(conn, _opts) do
    if Application.get_env(:council_hub_ui, :force_ssl_enabled, false) do
      Plug.SSL.call(conn, @ssl_opts)
    else
      conn
    end
  end

  @doc "Requests never redirected — local loopback access must stay plain http."
  def excluded?(conn), do: conn.host in ["localhost", "127.0.0.1", "::1", "[::1]"]
end
