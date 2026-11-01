defmodule CouncilHubUi.Repo.Migrations.AddArchiveAndLinkTypesToRooms do
  use Ecto.Migration

  # The Go server owns these columns in a real database; this keeps a fresh
  # (test) database in step with the read-only Room schema.
  def change do
    alter table(:rooms) do
      add :archived_at, :naive_datetime
      add :related_types, :string, default: ""
    end
  end
end
