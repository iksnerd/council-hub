defmodule CouncilHubUi.Repo.Migrations.CreateTables do
  use Ecto.Migration

  def change do
    create table(:rooms, primary_key: false) do
      add :id, :string, primary_key: true
      add :description, :string
      add :status, :string, default: "active"
      add :project, :string, default: ""
      add :tech_stack, :string, default: ""
      add :tags, :string, default: ""
      add :system_prompt, :string, default: ""
      add :related_rooms, :string, default: ""
      add :created_at, :naive_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :naive_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create table(:messages, primary_key: false) do
      add :id, :string, primary_key: true
      add :room_id, :string
      add :author, :string
      add :content, :string
      add :message_type, :string, default: "message"
      add :is_summary, :boolean, default: false
      add :reply_to, :string, default: ""
      add :pinned, :boolean, default: false
      add :timestamp, :naive_datetime, default: fragment("CURRENT_TIMESTAMP")
    end
  end
end
