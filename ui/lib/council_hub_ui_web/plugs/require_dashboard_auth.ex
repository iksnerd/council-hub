defmodule CouncilHubUiWeb.Plugs.RequireDashboardAuth do
  @moduledoc """
  HTTP Basic auth for every browser route, enabled only when
  `COUNCIL_AUTH_TOKEN` is set (`CouncilHubUi.Auth`). Any username is accepted;
  the password is the token.

  Missing or wrong credentials get a bare 401 with
  `WWW-Authenticate: Basic realm="Council Hub"` and an empty body. The plug sits
  first in the `:browser` pipeline, before the session is fetched, so a refused
  request gets no session cookie and no CSRF token. The LiveView socket `/live`
  needs the session and CSRF token embedded in an authenticated page, so it is
  protected implicitly.

  Requests whose `remote_ip` is loopback are exempt. That keeps the container
  healthcheck and the Go server's calls working. Traffic from outside the
  container arrives from the Docker bridge address, so a browser on the same
  host is not loopback and must log in.
  """
  import Plug.Conn

  alias CouncilHubUi.Auth
  alias CouncilHubUiWeb.Plugs.RestrictLocalhost

  def init(opts), do: opts

  def call(conn, _opts) do
    cond do
      not Auth.enabled?() -> conn
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
    |> send_resp(401, "")
    |> halt()
  end
end
