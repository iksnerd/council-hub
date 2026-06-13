defmodule CouncilHubUi.Repo.Migrations.AddStatusToNotebookEntries do
  use Ecto.Migration

  def change do
    alter table(:notebook_entries) do
      add :status, :string, default: "open"
    end
  end
end
