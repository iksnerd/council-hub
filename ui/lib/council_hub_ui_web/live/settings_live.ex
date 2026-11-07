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
  alias CouncilHubUi.McpClient
  import CouncilHubUiWeb.CouncilHelpers, only: [short_node: 1]

  @refresh_interval 3_000
  # After Sign in, check for the sign-in link every second for up to this long.
  @ts_poll_interval 1_000
  @ts_poll_max 40

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

  # One form serves "Sign in" and "Save settings". Signing in turns Tailscale on
  # with the form's values (the defaults on a first run); saving keeps it as it is.
  @impl true
  def handle_event("ts_apply", params, socket) do
    enabled = params["intent"] == "signin" or ts_enabled?(socket)

    attrs = %{
      "enabled" => enabled,
      "hostname" => String.trim(params["hostname"] || ""),
      "access" => params["access"],
      "control_url" => String.trim(params["control_url"] || ""),
      "tags" => parse_tags(params["tags"]),
      "auth_key" => String.trim(params["auth_key"] || "")
    }

    message = if enabled, do: "Tailscale started", else: "Tailscale settings saved"

    socket =
      socket
      |> ts_result(McpClient.tailscale_apply(attrs), message)
      |> then(fn socket ->
        # Only a Sign in click starts watching for the link; a plain save never opens a window.
        if params["intent"] == "signin", do: ts_follow_start(socket), else: socket
      end)

    {:noreply, socket}
  end

  # Cancel a sign-in in progress: stop the device, keep the settings.
  @impl true
  def handle_event("ts_cancel", _params, socket) do
    socket = ts_result(socket, McpClient.tailscale_apply(%{"enabled" => false}), "Tailscale off")
    {:noreply, assign(socket, :ts_awaiting, false)}
  end

  # Sign out: the saved login is removed and Tailscale goes off. The device stays
  # listed (offline) in the Tailscale admin console until deleted there.
  @impl true
  def handle_event("ts_signout", _params, socket) do
    socket = ts_result(socket, McpClient.tailscale_logout(), "Signed out of Tailscale")
    {:noreply, assign(socket, :ts_awaiting, false)}
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

  def handle_info(:ts_poll, socket) do
    socket = socket |> assign(:ts, McpClient.tailscale_status()) |> ts_follow()
    polls = (socket.assigns[:ts_polls] || 0) + 1

    cond do
      socket.assigns[:ts_awaiting] != true ->
        {:noreply, socket}

      polls >= @ts_poll_max ->
        {:noreply, assign(socket, ts_awaiting: false, ts_polls: polls)}

      true ->
        Process.send_after(self(), :ts_poll, @ts_poll_interval)
        {:noreply, assign(socket, :ts_polls, polls)}
    end
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
    |> assign_tailscale()
  end

  # The live status refreshes every few seconds; the form values are filled once
  # from the saved config and then only change when the operator saves, so a
  # refresh never overwrites what is being typed.
  defp assign_tailscale(socket) do
    ts = McpClient.tailscale_status()
    socket = socket |> assign(:ts, ts) |> assign_new(:ts_awaiting, fn -> false end)

    socket =
      case ts do
        {:ok, view} when not is_map_key(socket.assigns, :ts_loaded) ->
          socket |> assign(:ts_form, ts_form(view["config"])) |> assign(:ts_loaded, true)

        _ ->
          assign_new(socket, :ts_form, fn -> ts_form(%{}) end)
      end

    ts_follow(socket)
  end

  defp ts_form(config) do
    %{
      enabled: config["enabled"] == true,
      hostname: config["hostname"] || "council-hub",
      access: config["access"] || "owner",
      control_url: config["control_url"] || "",
      tags: Enum.join(config["tags"] || [], ", ")
    }
  end

  defp ts_result(socket, {:ok, view}, message) do
    socket
    |> put_flash(:info, message)
    |> assign(:ts, {:ok, view})
    |> assign(:ts_form, ts_form(view["config"]))
  end

  defp ts_result(socket, {:error, reason}, _message) do
    put_flash(socket, :error, "Tailscale: #{format_error(reason)}")
  end

  # After a Sign in click: watch for Tailscale's sign-in link and hand it to the
  # browser once, so the window the click opened can go to it. Only an https link
  # is ever sent. A page that merely loads while waiting for sign-in never starts
  # this, so reloading the page does not pop a window.
  defp ts_follow_start(socket) do
    if match?({:ok, _}, socket.assigns.ts) do
      Process.send_after(self(), :ts_poll, @ts_poll_interval)

      socket
      |> assign(:ts_awaiting, true)
      |> assign(:ts_polls, 0)
      |> ts_follow()
    else
      socket
    end
  end

  defp ts_follow(%{assigns: %{ts_awaiting: true, ts: {:ok, view}}} = socket) do
    case view do
      %{"state" => "needs-login", "auth_url" => "https://" <> _ = url} ->
        socket
        |> push_event("ts_auth_url", %{url: url})
        |> assign(:ts_awaiting, false)

      %{"state" => state} when state in ["starting", "needs-login"] ->
        socket

      _ ->
        assign(socket, :ts_awaiting, false)
    end
  end

  defp ts_follow(socket), do: socket

  defp ts_enabled?(socket) do
    match?({:ok, %{"config" => %{"enabled" => true}}}, socket.assigns[:ts])
  end

  defp parse_tags(nil), do: []

  defp parse_tags(str) do
    str |> String.split([",", " "], trim: true)
  end

  defp format_error(reason) when is_binary(reason), do: reason
  defp format_error(:econnrefused), do: "the Go server is not reachable"
  defp format_error(reason), do: inspect(reason)
end
