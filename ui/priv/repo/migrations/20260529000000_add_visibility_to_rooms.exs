defmodule CouncilHubUi.Repo.Migrations.AddVisibilityToRooms do
  use Ecto.Migration

  def change do
    alter table(:rooms) do
      add :visibility, :string, default: "public"
    end
  end
end
