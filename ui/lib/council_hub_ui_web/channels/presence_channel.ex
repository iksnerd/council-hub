defmodule CouncilHubUiWeb.PresenceChannel do
  @moduledoc """
  The `presence:agents` channel. A client joins with
  `%{"agent" => name, "client" => label, "rooms" => [room_id]}` and may send
  `"update"` with `%{"rooms" => [...]}` to replace the rooms it reports.
  Everything is client-reported and unverified.
  """

  use Phoenix.Channel

  alias CouncilHubUi.Presence

  @agent_re ~r/\A[A-Za-z0-9._@:-]{1,64}\z/
  @max_client 40
  @max_rooms 50
  @max_room_len 128

  @impl true
  def join("presence:agents", payload, socket) do
    with {:ok, agent, client, rooms} <- validate_join(payload),
         :ok <- check_capacity() do
      key = "#{agent}:#{Base.url_encode64(:crypto.strong_rand_bytes(9), padding: false)}"

      meta = %{
        agent: agent,
        client: client,
        rooms: rooms,
        node: Atom.to_string(Node.self()),
        joined_at: System.system_time(:second)
      }

      case Presence.track(self(), Presence.topic(), key, meta) do
        {:ok, _} -> {:ok, assign(socket, :presence_key, key)}
        _ -> {:error, %{reason: "presence_full"}}
      end
    else
      {:error, reason} -> {:error, %{reason: reason}}
    end
  end

  @impl true
  def handle_in("update", %{"rooms" => rooms}, socket) do
    with {:ok, rooms} <- validate_rooms(rooms) do
      Presence.update(
        self(),
        Presence.topic(),
        socket.assigns.presence_key,
        &%{&1 | rooms: rooms}
      )
    end

    {:noreply, socket}
  end

  def handle_in(_event, _payload, socket), do: {:noreply, socket}

  defp validate_join(%{"agent" => agent} = payload) when is_binary(agent) do
    if Regex.match?(@agent_re, agent) do
      client = Map.get(payload, "client", "")

      with true <- is_binary(client) and String.length(client) <= @max_client,
           {:ok, rooms} <- validate_rooms(Map.get(payload, "rooms", [])) do
        {:ok, agent, client, rooms}
      else
        _ -> {:error, "invalid_payload"}
      end
    else
      {:error, "invalid_agent"}
    end
  end

  defp validate_join(%{}), do: {:error, "invalid_agent"}
  defp validate_join(_), do: {:error, "invalid_payload"}

  defp validate_rooms(rooms) when is_list(rooms) and length(rooms) <= @max_rooms do
    if Enum.all?(rooms, &(is_binary(&1) and String.length(&1) <= @max_room_len)),
      do: {:ok, rooms},
      else: :error
  end

  defp validate_rooms(_), do: :error

  # Counts tracked connections cluster-wide. Concurrent joins can overshoot the
  # cap by a few; it bounds memory, it is not an exact quota.
  defp check_capacity do
    if Presence.connection_count() >= Presence.max_connections(),
      do: {:error, "presence_full"},
      else: :ok
  end
end
