defmodule CouncilHubUi.Repo.Migrations.AddReactionsToMessages do
  use Ecto.Migration

  def change do
    alter table(:messages) do
      add :reactions, :string, default: "{}"
    end
  end
end
