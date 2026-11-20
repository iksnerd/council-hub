defmodule CouncilHubUi.Council.AgentSeen do
  use Ecto.Schema

  # Read-only mirror of the Go server's agent_seen table: when each agent last
  # made a tool call on this node, and which tool. Go owns writes, and the table
  # is absent on a database created by an older Go server.
  @primary_key {:agent, :string, autogenerate: false}
  schema "agent_seen" do
    field :last_seen_at, :naive_datetime
    field :last_tool, :string, default: ""
  end
end
