defmodule CouncilHubUi.Repo.Migrations.AddRepoToRooms do
  use Ecto.Migration

  def change do
    alter table(:rooms) do
      add :repo, :string, default: ""
    end
  end
end
