defmodule CouncilHubUi.Repo.Migrations.AddMentionsToMessages do
  use Ecto.Migration

  def change do
    alter table(:messages) do
      add :mentions, :string, default: ""
    end
  end
end
