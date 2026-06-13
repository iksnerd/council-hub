defmodule CouncilHubUi.Repo.Migrations.CreateMessageLinks do
  use Ecto.Migration

  # Mirrors the Go server's message_links table (Go owns writes; the UI reads).
  def change do
    create_if_not_exists table(:message_links, primary_key: false) do
      add :id, :string, primary_key: true
      add :from_id, :string, null: false
      add :to_id, :string, null: false
      add :relation, :string, null: false
      add :author, :string, default: ""
      add :created_at, :naive_datetime
    end

    create_if_not_exists index(:message_links, [:from_id])
    create_if_not_exists index(:message_links, [:to_id])
  end
end
