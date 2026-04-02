defmodule CouncilHubUiWeb.Plugs.RestrictLocalhost do
  @moduledoc """
  Restricts access to localhost only. Used to protect internal API
  endpoints that should only be called by the co-located Go MCP server.
  """

  import Plug.Conn

  def init(opts), do: opts

  @loopback_v4 {127, 0, 0, 1}
  @loopback_v6 {0, 0, 0, 0, 0, 0, 0, 1}

  def call(conn, _opts) do
    if localhost?(conn.remote_ip) do
      conn
    else
      conn |> send_resp(403, "Forbidden") |> halt()
    end
  end

  defp localhost?(@loopback_v4), do: true
  defp localhost?(@loopback_v6), do: true
  # IPv4-mapped IPv6 loopback (::ffff:127.0.0.1 = {0,0,0,0,0,65535,32512,1})
  defp localhost?({0, 0, 0, 0, 0, 65535, 32512, 1}), do: true
  defp localhost?(_), do: false
end
