defmodule CouncilHubUi.Repo.Migrations.CreateSkills do
  use Ecto.Migration

  # Mirrors the Go server's skills table — the methodology registry (E3).
  # Go owns writes (register_skill); the UI reads. Production reads the
  # Go-created database file — this migration exists for dev/test parity only.
  def change do
    create_if_not_exists table(:skills, primary_key: false) do
      add :name, :string, primary_key: true
      add :description, :string, default: ""
      add :when_to_use, :string, default: ""
      add :content, :string, default: ""
      add :project, :string, default: ""
      add :tags, :string, default: ""
      add :source, :string, default: ""
      add :created_at, :naive_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :naive_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create_if_not_exists index(:skills, [:project])
  end
end
