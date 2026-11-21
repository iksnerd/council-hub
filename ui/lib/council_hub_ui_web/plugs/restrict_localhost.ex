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
    if loopback?(conn.remote_ip) do
      conn
    else
      conn |> send_resp(403, "Forbidden") |> halt()
    end
  end

  @doc "True for 127.0.0.1, ::1 and ::ffff:127.0.0.1."
  def loopback?(@loopback_v4), do: true
  def loopback?(@loopback_v6), do: true
  # IPv4-mapped IPv6 loopback (::ffff:127.0.0.1 = {0,0,0,0,0,65535,32512,1})
  def loopback?({0, 0, 0, 0, 0, 65535, 32512, 1}), do: true
  def loopback?(_), do: false
end
