defmodule CouncilHubUiWeb.Plugs.RequireClusterAdmin do
  @moduledoc """
  Gates the cluster settings page (a write surface) behind an admin token.

  IP-based "localhost only" cannot work behind Docker's bridge NAT — the
  container sees the gateway IP for all published-port traffic, so the host
  browser and a remote tailnet browser are indistinguishable. Instead:

  - If `COUNCIL_CLUSTER_ADMIN_TOKEN` is unset/empty, the page is disabled (404).
  - Visiting `/settings?token=<token>` with the correct token sets a session
    flag and redirects to the clean URL.
  - Once the session flag is set, access is allowed; otherwise 403.

  The token is compared in constant time. The session cookie is signed, so the
  flag cannot be forged client-side.
  """
  import Plug.Conn
  import Phoenix.Controller, only: [redirect: 2]

  def init(opts), do: opts

  def call(conn, _opts) do
    token = System.get_env("COUNCIL_CLUSTER_ADMIN_TOKEN")
    conn = fetch_query_params(conn)

    cond do
      token in [nil, ""] ->
        conn
        |> send_resp(
          404,
          "Cluster settings are disabled. Set COUNCIL_CLUSTER_ADMIN_TOKEN on the server to enable them."
        )
        |> halt()

      get_session(conn, :cluster_admin) == true ->
        conn

      is_binary(conn.params["token"]) and Plug.Crypto.secure_compare(conn.params["token"], token) ->
        conn
        |> put_session(:cluster_admin, true)
        |> redirect(to: "/settings")
        |> halt()

      true ->
        conn
        |> send_resp(
          403,
          "Forbidden. Append ?token=<COUNCIL_CLUSTER_ADMIN_TOKEN> to unlock cluster settings."
        )
        |> halt()
    end
  end
end
