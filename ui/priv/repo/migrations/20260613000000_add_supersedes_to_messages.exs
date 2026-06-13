defmodule CouncilHubUi.Repo.Migrations.AddSupersedesToMessages do
  use Ecto.Migration

  def change do
    alter table(:messages) do
      add :supersedes, :string, default: ""
    end
  end
end
