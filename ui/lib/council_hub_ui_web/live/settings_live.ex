defmodule CouncilHubUiWeb.SettingsLive do
  @moduledoc """
  Cluster settings page — the one write surface in the UI.

  Lets the operator connect/disconnect Erlang peer nodes live via
  `CouncilHubUi.ClusterManager`, no container restart. Gated behind an admin
  token: the router plug (RequireClusterAdmin) sets a signed-session flag on the
  dead render, and the websocket mount re-checks that same session flag.
  """
  use CouncilHubUiWeb, :live_view

  alias CouncilHubUi.ClusterManager
  import CouncilHubUiWeb.CouncilHelpers, only: [short_node: 1]

  require Logger

  @refresh_interval 3_000

  @impl true
  def mount(_params, session, socket) do
    if session["cluster_admin"] == true do
      if connected?(socket), do: Process.send_after(self(), :refresh, @refresh_interval)
      {:ok, assign_state(socket)}
    else
      {:ok,
       socket
       |> put_flash(:error, "Cluster settings require an admin token.")
       |> redirect(to: ~p"/")}
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
end
