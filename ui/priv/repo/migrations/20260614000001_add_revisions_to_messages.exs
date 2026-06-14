defmodule CouncilHubUi.Repo.Migrations.AddRevisionsToMessages do
  use Ecto.Migration

  # Append-only edits + retraction (the NLS Journal immutability property). The Go
  # server owns writes against the shared SQLite file; this mirrors its schema so the
  # read-only UI (and its test DB) can see the same columns.
  def change do
    alter table(:messages) do
      add :revises, :string, default: ""
      add :revised, :boolean, default: false
      add :retracted_at, :naive_datetime
      add :retracted_by, :string, default: ""
    end
  end
end
