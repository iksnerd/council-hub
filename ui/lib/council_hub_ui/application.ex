defmodule CouncilHubUi.Application do
  # See https://hexdocs.pm/elixir/Application.html
  # for more information on OTP Applications
  @moduledoc false

  use Application

  @impl true
  def start(_type, _args) do
    children = [
      CouncilHubUiWeb.Telemetry,
      CouncilHubUi.Repo,
      {DNSCluster, query: Application.get_env(:council_hub_ui, :dns_cluster_query) || :ignore},
      {Phoenix.PubSub, name: CouncilHubUi.PubSub},
      # Start a worker by calling: CouncilHubUi.Worker.start_link(arg)
      # {CouncilHubUi.Worker, arg},
      # Start to serve requests, typically the last entry
      CouncilHubUiWeb.Endpoint
    ]

    # See https://hexdocs.pm/elixir/Supervisor.html
    # for other strategies and supported options
    opts = [strategy: :one_for_one, name: CouncilHubUi.Supervisor]
    Supervisor.start_link(children, opts)
  end

  # Tell Phoenix to update the endpoint configuration
  # whenever the application is updated.
  @impl true
  def config_change(changed, _new, removed) do
    CouncilHubUiWeb.Endpoint.config_change(changed, removed)
    :ok
  end

end
