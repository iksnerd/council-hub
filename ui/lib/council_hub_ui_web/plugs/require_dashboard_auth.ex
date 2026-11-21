defmodule CouncilHubUiWeb.Plugs.RequireDashboardAuth do
  @moduledoc """
  HTTP Basic auth for every browser route, enabled only when
  `COUNCIL_AUTH_TOKEN` is set (`CouncilHubUi.Auth`). Any username is accepted;
  the password is the token.

  Missing or wrong credentials get a 401 with
  `WWW-Authenticate: Basic realm="Council Hub"` and a one-line plain-text body that
  says how to sign in (a browser shows the login prompt; the body is what you see
  if you cancel it, or what a script reads). It never says which part was wrong.
  The plug sits
  first in the `:browser` pipeline, before the session is fetched, so a refused
  request gets no session cookie and no CSRF token. The LiveView socket `/live`
  needs the session and CSRF token embedded in an authenticated page, so it is
  protected implicitly.

  Requests that came in through the trusted listener (`conn.private[:council_trusted]`,
  set by `Plugs.TrustedEntry`, see `CouncilHubUi.TrustedListener`) skip the check.

  Requests whose `remote_ip` is loopback are exempt. That keeps the container
  healthcheck and the Go server's calls working. Traffic from outside the
  container arrives from the Docker bridge address, so a browser on the same
  host is not loopback and must log in.
  """
  import Plug.Conn

  @denied_body "Authentication required. Sign in with any username and the hub's token as the password.\n"

  alias CouncilHubUi.Auth
  alias CouncilHubUiWeb.Plugs.RestrictLocalhost

  def init(opts), do: opts

  def call(conn, _opts) do
    cond do
      not Auth.enabled?() -> conn
      conn.private[:council_trusted] == true -> conn
      RestrictLocalhost.loopback?(conn.remote_ip) -> conn
      authorized?(conn) -> conn
      true -> deny(conn)
    end
  end

  defp authorized?(conn) do
    with [header | _] <- get_req_header(conn, "authorization"),
         "Basic " <> encoded <- header,
         {:ok, decoded} <- Base.decode64(String.trim(encoded)),
         [_user, password] <- String.split(decoded, ":", parts: 2) do
      Auth.valid?(password)
    else
      _ -> false
    end
  end

  defp deny(conn) do
    conn
    |> put_resp_header("www-authenticate", ~s(Basic realm="Council Hub"))
    |> put_resp_content_type("text/plain")
    |> send_resp(401, @denied_body)
    |> halt()
  end
end
