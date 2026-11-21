defmodule CouncilHubUiWeb.Plugs.TrustedEntry do
  @moduledoc """
  Entry plug of the trusted listener (`CouncilHubUi.TrustedListener`).

  Deletes any client-supplied `x-council-trusted` header, marks the connection
  trusted in `conn.private` and adds the header with the per-boot secret, then
  calls the endpoint. `conn.private` cannot be set by a client; the header is
  for `UserSocket.connect/3`, which only sees request headers.
  """
  import Plug.Conn

  alias CouncilHubUi.TrustedListener

  def init(opts), do: opts

  def call(conn, opts) do
    endpoint = Keyword.get(opts, :endpoint, CouncilHubUiWeb.Endpoint)

    conn
    |> delete_req_header(TrustedListener.header())
    |> put_private(:council_trusted, true)
    |> add_marker()
    |> endpoint.call(endpoint.init([]))
  end

  defp add_marker(conn) do
    marker = {TrustedListener.header(), TrustedListener.secret()}
    %{conn | req_headers: [marker | conn.req_headers]}
  end
end
