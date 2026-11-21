defmodule CouncilHubUi.Application do
  # See https://hexdocs.pm/elixir/Application.html
  # for more information on OTP Applications
  @moduledoc false

  use Application

  @impl true
  def start(_type, _args) do
    topologies = Application.get_env(:libcluster, :topologies) || []

    # Notices inbound bad-cookie rejections so /health can say "cookie_mismatch".
    CouncilHubUi.CookieWatch.install()

    children = [
      CouncilHubUiWeb.Telemetry,
      CouncilHubUi.Repo,
      {DNSCluster, query: Application.get_env(:council_hub_ui, :dns_cluster_query) || :ignore},
      {Cluster.Supervisor, [topologies, [name: CouncilHubUi.ClusterSupervisor]]},
      CouncilHubUi.ClusterManager,
      {Phoenix.PubSub, name: CouncilHubUi.PubSub}
    ]

    children =
      children ++
        presence_children() ++
        [
          # Start to serve requests, typically the last entry
          CouncilHubUiWeb.Endpoint
        ] ++ trusted_children()

    # See https://hexdocs.pm/elixir/Supervisor.html
    # for other strategies and supported options
    opts = [strategy: :one_for_one, name: CouncilHubUi.Supervisor]
    Supervisor.start_link(children, opts)
  end

  # The second, token-free listener (COUNCIL_TRUSTED_PORT), serving the same
  # Endpoint. Only when the Endpoint itself serves.
  defp trusted_children do
    if CouncilHubUi.TrustedListener.enabled?() do
      # Make the per-boot secret before the first request.
      CouncilHubUi.TrustedListener.secret()

      unless CouncilHubUi.Auth.enabled?() do
        require Logger

        Logger.warning(
          "COUNCIL_TRUSTED_PORT is set while COUNCIL_AUTH_TOKEN is empty: " <>
            "the trusted listener adds nothing while auth is off."
        )
      end

      [CouncilHubUi.TrustedListener]
    else
      []
    end
  end

  # Presence needs PubSub and is skipped entirely with COUNCIL_PRESENCE=off.
  defp presence_children do
    if CouncilHubUi.Presence.enabled?(), do: [CouncilHubUi.Presence], else: []
  end

  # Tell Phoenix to update the endpoint configuration
  # whenever the application is updated.
  @impl true
  def config_change(changed, _new, removed) do
    CouncilHubUiWeb.Endpoint.config_change(changed, removed)
    :ok
  end
end
