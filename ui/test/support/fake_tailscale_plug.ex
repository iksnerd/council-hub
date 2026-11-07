defmodule CouncilHubUi.FakeTailscalePlug do
  @moduledoc false
  # Stands in for the Go server's /api/ui/tailscale endpoint in tests. The reply
  # and the last request body are held in an Agent named by the :agent option.
  import Plug.Conn

  def init(opts), do: opts

  def call(%{request_path: "/api/ui/tailscale"} = conn, agent: agent) do
    {:ok, body, conn} = read_body(conn)
    Agent.update(agent, &Map.put(&1, :last, {conn.method, body}))
    {status, reply} = Agent.get(agent, & &1.reply)

    conn
    |> put_resp_content_type("application/json")
    |> send_resp(status, reply)
  end

  def call(conn, _), do: send_resp(conn, 404, "not found")
end
