defmodule CouncilHubUi.Repo.Migrations.AddDeletedAtToNotebooks do
  use Ecto.Migration

  # Mirrors the Go server's soft delete (notebooks.deleted_at). Dev/test parity only.
  def change do
    alter table(:notebooks) do
      add :deleted_at, :naive_datetime
    end
  end
end
