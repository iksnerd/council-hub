defmodule CouncilHubUiWeb.Plugs.RestrictLocalhost do
  @moduledoc """
  Restricts access to localhost only. Used to protect internal API
  endpoints that should only be called by the co-located Go MCP server.
  """

  import Plug.Conn

  def init(opts), do: opts

  def call(conn, _opts) do
    case conn.remote_ip do
      {127, 0, 0, 1} -> conn
      {0, 0, 0, 0, 0, 0, 0, 1} -> conn
      _ -> conn |> send_resp(403, "Forbidden") |> halt()
    end
  end
end
