defmodule CouncilHubUiWeb.PresenceChannelTest do
  # Not async: presence is one global tracker and the cap is app env.
  use ExUnit.Case, async: false

  import Phoenix.ChannelTest

  alias CouncilHubUi.Presence
  alias CouncilHubUiWeb.{PresenceChannel, UserSocket}

  @endpoint CouncilHubUiWeb.Endpoint
  @topic "presence:agents"

  setup do
    on_exit(fn ->
      Application.delete_env(:council_hub_ui, :presence_max_connections)
      Application.delete_env(:council_hub_ui, :presence_enabled)
    end)

    {:ok, socket} = connect(UserSocket, %{})
    {:ok, socket: socket}
  end

  defp join_as(socket, payload), do: subscribe_and_join(socket, PresenceChannel, @topic, payload)

  defp entries(agent) do
    for {key, %{metas: [meta]}} <- Presence.list(@topic), meta.agent == agent, do: {key, meta}
  end

  test "a valid join tracks a per-connection key with the right meta", %{socket: socket} do
    {:ok, %{}, _} =
      join_as(socket, %{"agent" => "pc-alice", "client" => "council-inbox", "rooms" => ["r1"]})

    assert [{key, meta}] = entries("pc-alice")
    assert String.starts_with?(key, "pc-alice:")
    assert meta.client == "council-inbox"
    assert meta.rooms == ["r1"]
    assert meta.node == Atom.to_string(Node.self())
    assert is_integer(meta.joined_at)
  end

  test "two connections of one agent get distinct keys", %{socket: socket} do
    {:ok, _, _} = join_as(socket, %{"agent" => "pc-twin", "client" => "codex-cli", "rooms" => []})
    {:ok, s2} = connect(UserSocket, %{})

    {:ok, _, _} =
      subscribe_and_join(s2, PresenceChannel, @topic, %{
        "agent" => "pc-twin",
        "client" => "gemini-cli",
        "rooms" => []
      })

    assert length(entries("pc-twin")) == 2
  end

  test "joins reply with the contract's error reasons", %{socket: socket} do
    bad = fn payload -> join_as(socket, payload) end

    assert {:error, %{reason: "invalid_agent"}} = bad.(%{"agent" => "bad name!", "rooms" => []})
    assert {:error, %{reason: "invalid_agent"}} = bad.(%{"agent" => String.duplicate("a", 65)})
    assert {:error, %{reason: "invalid_agent"}} = bad.(%{"client" => "x"})
    assert {:error, %{reason: "invalid_agent"}} = bad.(%{"agent" => 5})

    ok = %{"agent" => "pc-ok", "client" => "c", "rooms" => []}

    assert {:error, %{reason: "invalid_payload"}} =
             bad.(%{ok | "client" => String.duplicate("c", 41)})

    assert {:error, %{reason: "invalid_payload"}} = bad.(%{ok | "rooms" => "r1"})

    assert {:error, %{reason: "invalid_payload"}} =
             bad.(%{ok | "rooms" => Enum.map(1..51, &"r#{&1}")})

    assert {:error, %{reason: "invalid_payload"}} =
             bad.(%{ok | "rooms" => [String.duplicate("r", 129)]})

    assert {:error, %{reason: "invalid_payload"}} = bad.(%{ok | "rooms" => [1]})
    assert entries("pc-ok") == []
  end

  test "update replaces the tracked rooms; bad updates and unknown events are ignored", %{
    socket: socket
  } do
    {:ok, _, ch} = join_as(socket, %{"agent" => "pc-upd", "client" => "c", "rooms" => ["a"]})

    push(ch, "update", %{"rooms" => ["b", "c"]})
    push(ch, "update", %{"rooms" => "nope"})
    push(ch, "mystery", %{"x" => 1})

    wait_until(fn -> match?([{_, %{rooms: ["b", "c"]}}], entries("pc-upd")) end)
    assert Process.alive?(ch.channel_pid)
  end

  test "leaving removes the entry", %{socket: socket} do
    {:ok, _, ch} = join_as(socket, %{"agent" => "pc-leave", "client" => "c", "rooms" => []})
    assert [_] = entries("pc-leave")

    Process.unlink(ch.channel_pid)
    ref = leave(ch)
    assert_reply ref, :ok
    wait_until(fn -> entries("pc-leave") == [] end)
  end

  test "the join over the cap is rejected with presence_full", %{socket: socket} do
    Application.put_env(
      :council_hub_ui,
      :presence_max_connections,
      Presence.connection_count() + 1
    )

    {:ok, _, _} = join_as(socket, %{"agent" => "pc-cap1", "client" => "c", "rooms" => []})
    {:ok, s2} = connect(UserSocket, %{})

    assert {:error, %{reason: "presence_full"}} =
             subscribe_and_join(s2, PresenceChannel, @topic, %{"agent" => "pc-cap2"})

    assert entries("pc-cap2") == []
  end

  test "with presence off the socket refuses connections and nothing is listed" do
    Application.put_env(:council_hub_ui, :presence_enabled, false)
    assert :error = connect(UserSocket, %{})
    assert Presence.agents() == []
  end

  test "COUNCIL_PRESENCE parsing: only off/0/false/no disable it" do
    assert Presence.parse_enabled(nil)
    assert Presence.parse_enabled("on")
    assert Presence.parse_enabled("")
    refute Presence.parse_enabled("off")
    refute Presence.parse_enabled(" OFF ")
    refute Presence.parse_enabled("0")
    refute Presence.parse_enabled("false")
  end

  test "agents/0 groups connections by agent with count, clients, rooms", %{socket: socket} do
    {:ok, _, _} =
      join_as(socket, %{"agent" => "pc-grp", "client" => "codex-cli", "rooms" => ["x"]})

    {:ok, s2} = connect(UserSocket, %{})

    {:ok, _, _} =
      subscribe_and_join(s2, PresenceChannel, @topic, %{
        "agent" => "pc-grp",
        "client" => "gemini-cli",
        "rooms" => ["y", "x"]
      })

    assert %{count: 2, clients: ["codex-cli", "gemini-cli"], rooms: ["x", "y"]} =
             Enum.find(Presence.agents(), &(&1.agent == "pc-grp"))
  end

  defp wait_until(fun, tries \\ 50) do
    cond do
      fun.() ->
        :ok

      tries == 0 ->
        flunk("condition not met")

      true ->
        Process.sleep(20)
        wait_until(fun, tries - 1)
    end
  end
end
