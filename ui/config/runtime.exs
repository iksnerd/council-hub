import Config

# config/runtime.exs is executed for all environments, including
# during releases. It is executed after compilation and before the
# system starts, so it is typically used to load production configuration
# and secrets from environment variables or elsewhere. Do not define
# any compile-time configuration in here, as it won't be applied.
# The block below contains prod specific runtime configuration.

# ## Using releases
#
# If you use `mix release`, you need to explicitly enable the server
# by passing the PHX_SERVER=true when you start it:
#
#     PHX_SERVER=true bin/council_hub_ui start
#
# Alternatively, you can use `mix phx.gen.release` to generate a `bin/server`
# script that automatically sets the env var above.
if System.get_env("PHX_SERVER") do
  config :council_hub_ui, CouncilHubUiWeb.Endpoint, server: true
end

# Clustering: if COUNCIL_SEEDS is set (comma-separated node names like
# "council_hub@10.0.0.5,council_hub@10.0.0.6"), use Epmd strategy to
# connect to those specific nodes. Otherwise fall back to Gossip for
# automatic LAN/multicast discovery.
cluster_topology =
  case System.get_env("COUNCIL_SEEDS") do
    nil ->
      [council_hub: [strategy: Cluster.Strategy.Gossip]]

    "" ->
      [council_hub: [strategy: Cluster.Strategy.Gossip]]

    seeds_str ->
      seeds =
        seeds_str
        |> String.split(",", trim: true)
        |> Enum.map(fn s -> String.to_atom(String.trim(s)) end)

      [council_hub: [
        strategy: Cluster.Strategy.Epmd,
        config: [hosts: seeds]
      ]]
  end

config :libcluster, topologies: cluster_topology

config :council_hub_ui, CouncilHubUiWeb.Endpoint,
  http: [port: String.to_integer(System.get_env("PORT", "4000"))]

if config_env() == :prod do
  database_path =
    System.get_env("COUNCIL_DB_PATH") ||
      raise """
      environment variable COUNCIL_DB_PATH is missing.
      For example: /data/council.db
      """

  config :council_hub_ui, CouncilHubUi.Repo,
    database: database_path,
    pool_size: String.to_integer(System.get_env("POOL_SIZE") || "5"),
    journal_mode: :wal

  # The secret key base is used to sign/encrypt cookies and other secrets.
  # A default value is used in config/dev.exs and config/test.exs but you
  # want to use a different value for prod and you most likely don't want
  # to check this value into version control, so we use an environment
  # variable instead.
  secret_key_base =
    System.get_env("SECRET_KEY_BASE") ||
      raise """
      environment variable SECRET_KEY_BASE is missing.
      You can generate one by calling: mix phx.gen.secret
      """

  host = System.get_env("PHX_HOST") || "localhost"
  port = String.to_integer(System.get_env("PORT", "4000"))
  scheme = if host == "localhost", do: "http", else: "https"

  config :council_hub_ui, :dns_cluster_query, System.get_env("DNS_CLUSTER_QUERY")

  config :council_hub_ui, CouncilHubUiWeb.Endpoint,
    url: [host: host, port: port, scheme: scheme],
    http: [
      ip: {0, 0, 0, 0, 0, 0, 0, 0},
      port: port
    ],
    secret_key_base: secret_key_base

  # ## SSL Support
  #
  # To get SSL working, you will need to add the `https` key
  # to your endpoint configuration:
  #
  #     config :council_hub_ui, CouncilHubUiWeb.Endpoint,
  #       https: [
  #         ...,
  #         port: 443,
  #         cipher_suite: :strong,
  #         keyfile: System.get_env("SOME_APP_SSL_KEY_PATH"),
  #         certfile: System.get_env("SOME_APP_SSL_CERT_PATH")
  #       ]
  #
  # The `cipher_suite` is set to `:strong` to support only the
  # latest and more secure SSL ciphers. This means old browsers
  # and clients may not be supported. You can set it to
  # `:compatible` for wider support.
  #
  # `:keyfile` and `:certfile` expect an absolute path to the key
  # and cert in disk or a relative path inside priv, for example
  # "priv/ssl/server.key". For all supported SSL configuration
  # options, see https://hexdocs.pm/plug/Plug.SSL.html#configure/1
  #
  # We also recommend setting `force_ssl` in your config/prod.exs,
  # ensuring no data is ever sent via http, always redirecting to https:
  #
  #     config :council_hub_ui, CouncilHubUiWeb.Endpoint,
  #       force_ssl: [hsts: true]
  #
  # Check `Plug.SSL` for all available options in `force_ssl`.
end
