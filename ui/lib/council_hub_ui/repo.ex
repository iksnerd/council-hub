defmodule CouncilHubUi.Repo do
  use Ecto.Repo,
    otp_app: :council_hub_ui,
    adapter: Ecto.Adapters.SQLite3
end
