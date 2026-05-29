defmodule CouncilHubUi.ClusterManager do
  @moduledoc """
  Runtime cluster membership management.

  Lets the dashboard connect/disconnect Erlang peer nodes live — no container
  restart — using `Node.connect/1` / `Node.disconnect/1`. Managed peers are
  persisted to a file in the data volume so they are re-connected on the next
  boot, complementing (not replacing) the libcluster `COUNCIL_SEEDS` strategy.

  This is the only write path in the otherwise read-only UI, and the settings
  page that drives it is gated to localhost.
  """
  use GenServer
  require Logger

  # name@host — host may be an IP, hostname, or Tailscale MagicDNS name.
  @node_re ~r/^[^@\s]+@[^@\s]+$/

  ## Client API

  def start_link(opts) do
    name = Keyword.get(opts, :name, __MODULE__)
    GenServer.start_link(__MODULE__, opts, name: name)
  end

  @doc "Connect to a peer node (string like `council_hub@100.x.y.z`) and persist it."
  def connect(server \\ __MODULE__, node_str) when is_binary(node_str) do
    GenServer.call(server, {:connect, String.trim(node_str)})
  end

  @doc "Disconnect a peer node and drop it from the persisted set."
  def disconnect(server \\ __MODULE__, node_str) when is_binary(node_str) do
    GenServer.call(server, {:disconnect, String.trim(node_str)})
  end

  @doc "List the node names this manager is responsible for (persisted), as strings."
  def managed_peers(server \\ __MODULE__) do
    GenServer.call(server, :managed_peers)
  end

  ## Server callbacks

  @impl true
  def init(opts) do
    path = Keyword.get(opts, :path) || default_path()
    peers = read_peers(path)

    # Re-establish connections from the persisted set on boot.
    for node <- peers do
      case safe_connect(node) do
        true ->
          Logger.info("ClusterManager: reconnected to #{node}")

        other ->
          Logger.warning("ClusterManager: could not reconnect to #{node} (#{inspect(other)})")
      end
    end

    {:ok, %{path: path, peers: peers}}
  end

  @impl true
  def handle_call({:connect, node_str}, _from, state) do
    with :ok <- validate(node_str, state),
         node <- String.to_atom(node_str),
         true <- safe_connect(node) do
      peers = MapSet.put(state.peers, node_str)
      write_peers(state.path, peers)
      {:reply, :ok, %{state | peers: peers}}
    else
      {:error, reason} ->
        {:reply, {:error, reason}, state}

      false ->
        {:reply,
         {:error,
          "could not reach #{node_str} — check the IP, that it's running, and the cookie matches"},
         state}

      :ignored ->
        {:reply,
         {:error,
          "this node is not running in distributed mode (no RELEASE_NODE) — cannot connect peers"},
         state}
    end
  end

  @impl true
  def handle_call({:disconnect, node_str}, _from, state) do
    Node.disconnect(String.to_atom(node_str))
    peers = MapSet.delete(state.peers, node_str)
    write_peers(state.path, peers)
    {:reply, :ok, %{state | peers: peers}}
  end

  @impl true
  def handle_call(:managed_peers, _from, state) do
    {:reply, MapSet.to_list(state.peers), state}
  end

  ## Helpers

  defp validate(node_str, state) do
    cond do
      not Regex.match?(@node_re, node_str) ->
        {:error, "invalid node name — expected something like council_hub@100.x.y.z"}

      node_str == to_string(Node.self()) ->
        {:error, "that's this node — pick a peer's node name"}

      MapSet.member?(state.peers, node_str) and
          Enum.member?(Node.list(), String.to_atom(node_str)) ->
        {:error, "already connected to #{node_str}"}

      true ->
        :ok
    end
  end

  # Node.connect returns true | false | :ignored (when not distributed).
  defp safe_connect(node) when is_atom(node), do: Node.connect(node)
  defp safe_connect(node) when is_binary(node), do: Node.connect(String.to_atom(node))

  defp read_peers(path) do
    case File.read(path) do
      {:ok, contents} ->
        contents
        |> String.split("\n", trim: true)
        |> Enum.map(&String.trim/1)
        |> Enum.reject(&(&1 == ""))
        |> MapSet.new()

      {:error, _} ->
        MapSet.new()
    end
  end

  defp write_peers(path, peers) do
    contents = peers |> MapSet.to_list() |> Enum.sort() |> Enum.join("\n")

    with :ok <- File.mkdir_p(Path.dirname(path)),
         :ok <- File.write(path, contents) do
      :ok
    else
      {:error, reason} ->
        Logger.warning("ClusterManager: could not persist peers to #{path}: #{inspect(reason)}")
        :ok
    end
  end

  @doc "Default peers file: alongside the SQLite DB, else a tmp fallback."
  def default_path do
    case System.get_env("COUNCIL_DB_PATH") do
      nil -> Path.join(System.tmp_dir!(), "council_hub_cluster_peers")
      db_path -> Path.join(Path.dirname(db_path), "cluster_peers")
    end
  end
end
