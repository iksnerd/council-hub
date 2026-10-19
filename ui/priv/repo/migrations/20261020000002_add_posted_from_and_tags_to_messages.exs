defmodule CouncilHubUi.Repo.Migrations.AddPostedFromAndTagsToMessages do
  use Ecto.Migration

  # Mirrors the Go server's messages.posted_from / msg_tags (v0.63.0). Dev/test parity only.
  def change do
    alter table(:messages) do
      add :posted_from, :string, default: ""
      add :msg_tags, :string, default: ""
    end
  end
end
