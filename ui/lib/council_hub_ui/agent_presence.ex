defmodule CouncilHubUi.AgentPresence do
  @moduledoc """
  "Last seen" per agent, read from the Go server's `agent_seen` table. It only
  reflects tool calls this node received (node-local), not whether an agent is
  running. A missing table (older Go server) reads as an empty list.
  """

  import Ecto.Query
  require Logger

  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.AgentSeen

  @window_seconds 24 * 3600

  @doc "Agents seen in the last 24h, newest first, capped at `limit`."
  def recent(limit \\ 12, now \\ NaiveDateTime.utc_now()) do
    cutoff = NaiveDateTime.add(now, -@window_seconds, :second)

    Repo.all(
      from s in AgentSeen,
        where: s.last_seen_at > ^cutoff,
        order_by: [desc: s.last_seen_at, asc: s.agent],
        limit: ^limit
    )
  rescue
    e ->
      Logger.debug("agent_seen unavailable: #{Exception.message(e)}")
      []
  end
end
