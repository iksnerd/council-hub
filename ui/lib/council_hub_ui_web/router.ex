defmodule CouncilHubUiWeb.Router do
  use CouncilHubUiWeb, :router

  pipeline :browser do
    plug :accepts, ["html"]
    plug :fetch_session
    plug :fetch_live_flash
    plug :put_root_layout, html: {CouncilHubUiWeb.Layouts, :root}
    plug :protect_from_forgery
    plug :put_secure_browser_headers
  end

  pipeline :api do
    plug :accepts, ["json"]
  end

  pipeline :internal_api do
    plug :accepts, ["json"]
    plug CouncilHubUiWeb.Plugs.RestrictLocalhost
  end

  # Cluster controls are a write surface — gate behind an admin token.
  # (IP-based localhost gating can't work behind Docker NAT; see the plug.)
  pipeline :require_cluster_admin do
    plug CouncilHubUiWeb.Plugs.RequireClusterAdmin
  end

  scope "/", CouncilHubUiWeb do
    pipe_through :browser

    get "/rooms/:room_id/export", RoomController, :export

    live_session :default do
      live "/", CouncilLive, :index
      live "/status", StatusLive, :index
      live "/notebook", NotebookLive, :index
      live "/skills", SkillsLive, :index
      live "/rooms/:room_id", CouncilLive, :show
    end
  end

  scope "/", CouncilHubUiWeb do
    pipe_through [:browser, :require_cluster_admin]

    live_session :settings do
      live "/settings", SettingsLive, :index
    end
  end

  scope "/api/internal/cluster", CouncilHubUiWeb do
    pipe_through :internal_api

    get "/nodes", ClusterController, :nodes
    post "/search_messages", ClusterController, :search_messages
    post "/list_rooms", ClusterController, :list_rooms
    post "/room_stats", ClusterController, :room_stats
    post "/read_transcript", ClusterController, :read_transcript
    post "/get_messages", ClusterController, :get_messages
    post "/get_digest", ClusterController, :get_digest
    post "/read_notebook", ClusterController, :read_notebook
    post "/locate_room", ClusterController, :locate_room
  end
end
