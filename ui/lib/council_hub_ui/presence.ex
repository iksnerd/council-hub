defmodule CouncilHubUi.Presence do
  @moduledoc """
  Live agent presence on the topic `presence:agents`, backed by
  `Phoenix.Presence` (cluster-wide through `CouncilHubUi.PubSub`).

  Any process that joins the topic over `/socket` is tracked, for example
  `council-inbox watch`, a generic sidecar any agent harness can run. The key
  is per connection (`"<agent>:<random>"`) because many sessions share one agent
  name. Meta: `%{agent, client, rooms, node, joined_at}`.

  Everything here is reported by the client and not verified: the socket is
  reachable like the rest of the dashboard, with no auth, so a client can claim
  any name. Presence is informational only. It cannot read or write messages.

  `COUNCIL_PRESENCE=off` starts no tracker and the socket refuses connections.
  A node that does not run presence (off, or an older release) neither tracks
  nor shows presence entries.
  """

  use Phoenix.Presence,
    otp_app: :council_hub_ui,
    pubsub_server: CouncilHubUi.PubSub

  @topic "presence:agents"
  @default_max_connections 500

  def topic, do: @topic

  @doc "Is presence enabled on this node? (config :council_hub_ui, :presence_enabled; default true)"
  def enabled?, do: Application.get_env(:council_hub_ui, :presence_enabled, true)

  @doc "Maximum tracked connections across the cluster (default 500)."
  def max_connections,
    do: Application.get_env(:council_hub_ui, :presence_max_connections, @default_max_connections)

  @doc "Parses `COUNCIL_PRESENCE`: only off/0/false/no (any case) disable it."
  def parse_enabled(nil), do: true

  def parse_enabled(value) when is_binary(value),
    do: String.downcase(String.trim(value)) not in ["off", "0", "false", "no"]

  @doc "Number of tracked connections."
  def connection_count, do: map_size(list(@topic))

  @doc """
  Live connections grouped by agent, sorted by agent name. `[]` when presence is
  off on this node.
  """
  def agents do
    if enabled?() do
      @topic
      |> list()
      |> Enum.flat_map(fn {_key, %{metas: metas}} -> metas end)
      |> Enum.group_by(& &1.agent)
      |> Enum.map(fn {agent, metas} ->
        %{
          agent: agent,
          count: length(metas),
          clients:
            metas |> Enum.map(& &1.client) |> Enum.reject(&(&1 in [nil, ""])) |> uniq_sort(),
          rooms: metas |> Enum.flat_map(& &1.rooms) |> uniq_sort(),
          nodes: metas |> Enum.map(& &1.node) |> uniq_sort()
        }
      end)
      |> Enum.sort_by(& &1.agent)
    else
      []
    end
  end

  defp uniq_sort(list), do: list |> Enum.uniq() |> Enum.sort()
end
