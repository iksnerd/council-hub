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

  scope "/", CouncilHubUiWeb do
    pipe_through :browser

    get "/rooms/:room_id/export", RoomController, :export

    live_session :default do
      live "/", CouncilLive, :index
      live "/rooms/:room_id", CouncilLive, :show
    end
  end

  scope "/api/internal/cluster", CouncilHubUiWeb do
    pipe_through :internal_api

    post "/search_messages", ClusterController, :search_messages
    post "/list_rooms", ClusterController, :list_rooms
    post "/room_stats", ClusterController, :room_stats
  end
end
