defmodule CouncilHubUi.Repo.Migrations.AddWorkItemsToRooms do
  use Ecto.Migration

  def change do
    alter table(:rooms) do
      add :work_items, :string, default: ""
    end
  end
end
