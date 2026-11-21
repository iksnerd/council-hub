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

  @doc """
  Rows for the Agents panel: live connections (from `CouncilHubUi.Presence`,
  reported by the client, not verified) first, sorted by name, then agents with no
  live connection in `seen` order ("last seen"). An agent that is both live and
  in `seen` gets one row carrying both.
  """
  def merge(live, seen) do
    seen_by_agent = Map.new(seen, &{&1.agent, &1})
    live_names = MapSet.new(live, & &1.agent)

    live_rows =
      Enum.map(live, fn l ->
        s = Map.get(seen_by_agent, l.agent)

        Map.merge(l, %{
          live: true,
          last_seen_at: s && s.last_seen_at,
          last_tool: s && s.last_tool
        })
      end)

    seen_rows =
      for s <- seen, not MapSet.member?(live_names, s.agent) do
        %{
          agent: s.agent,
          live: false,
          count: 0,
          clients: [],
          rooms: [],
          nodes: [],
          last_seen_at: s.last_seen_at,
          last_tool: s.last_tool
        }
      end

    live_rows ++ seen_rows
  end

  @doc "Merged panel rows from current presence and the given last-seen rows."
  def panel_rows(seen), do: merge(CouncilHubUi.Presence.agents(), seen)

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
