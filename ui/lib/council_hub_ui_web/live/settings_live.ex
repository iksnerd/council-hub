defmodule CouncilHubUiWeb.SettingsLive do
  @moduledoc """
  Cluster settings page — the one write surface in the UI.

  Lets the host connect/disconnect Erlang peer nodes live via
  `CouncilHubUi.ClusterManager`, no container restart. Gated to localhost at
  both the router (dead render) and here (websocket) via `:peer_data`.
  """
  use CouncilHubUiWeb, :live_view

  alias CouncilHubUi.ClusterManager
  import CouncilHubUiWeb.CouncilHelpers, only: [short_node: 1]

  require Logger

  @refresh_interval 3_000

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket) do
      case allowed?(socket) do
        true -> Process.send_after(self(), :refresh, @refresh_interval)
        false -> nil
      end

      if !allowed?(socket) do
        {:ok,
         socket
         |> put_flash(:error, "Cluster settings are restricted to localhost.")
         |> redirect(to: ~p"/")}
      else
        {:ok, assign_state(socket)}
      end
    else
      {:ok, assign_state(socket)}
    end
  end

  @impl true
  def handle_event("connect", %{"node" => node}, socket) do
    case ClusterManager.connect(node) do
      :ok ->
        {:noreply,
         socket
         |> put_flash(:info, "Connected to #{node}")
         |> assign(:node_input, "")
         |> assign_state()}

      {:error, reason} ->
        {:noreply, put_flash(socket, :error, reason)}
    end
  end

  @impl true
  def handle_event("disconnect", %{"node" => node}, socket) do
    :ok = ClusterManager.disconnect(node)

    {:noreply,
     socket
     |> put_flash(:info, "Disconnected #{node}")
     |> assign_state()}
  end

  @impl true
  def handle_event("change", %{"node" => node}, socket) do
    {:noreply, assign(socket, :node_input, node)}
  end

  @impl true
  def handle_info(:refresh, socket) do
    Process.send_after(self(), :refresh, @refresh_interval)
    {:noreply, assign_state(socket)}
  end

  ## Helpers

  defp assign_state(socket) do
    self_node = to_string(Node.self())
    connected = Node.list() |> Enum.map(&to_string/1) |> Enum.sort()
    managed = ClusterManager.managed_peers() |> Enum.sort()

    socket
    |> assign(:page_title, "Cluster Settings")
    |> assign(:self_node, self_node)
    |> assign(:distributed?, self_node != "nonode@nohost")
    |> assign(:cookie_set?, System.get_env("RELEASE_COOKIE") not in [nil, ""])
    |> assign(:connected_nodes, connected)
    |> assign(:managed_peers, managed)
    |> assign_new(:node_input, fn -> "" end)
    |> assign(:short_node_fun, &short_node/1)
  end

  defp allowed?(socket) do
    case get_connect_info(socket, :peer_data) do
      %{address: address} -> loopback?(address)
      # No peer_data (e.g. test/longpoll without it) — the router plug already
      # gated the dead render, so don't lock out legitimate connections.
      _ -> true
    end
  end

  defp loopback?({127, _, _, _}), do: true
  defp loopback?({0, 0, 0, 0, 0, 0, 0, 1}), do: true
  defp loopback?({0, 0, 0, 0, 0, 65535, 32512, 1}), do: true
  defp loopback?(_), do: false
end
