defmodule CouncilHubUi.Repo.Migrations.CreateNotebooks do
  use Ecto.Migration

  # Mirrors the Go server's notebooks schema (mcp-server/internal/council/db.go).
  # Production reads the Go-created database file — this migration exists for
  # dev/test parity only, like the rest of priv/repo/migrations.
  def change do
    create table(:notebooks, primary_key: false) do
      add :id, :string, primary_key: true
      add :project, :string, default: ""
      add :title, :string, default: ""
      add :created_at, :naive_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :naive_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create table(:notebook_entries, primary_key: false) do
      add :id, :string, primary_key: true
      add :notebook_id, :string
      add :position, :integer
      add :kind, :string, default: "prose"
      add :ref_id, :string, default: ""
      add :prose, :string, default: ""
      add :created_at, :naive_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create index(:notebook_entries, [:notebook_id, :position])
    create index(:notebooks, [:project])
  end
end
