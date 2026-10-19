defmodule CouncilHubUi.Council.Notebook do
  use Ecto.Schema

  @primary_key {:id, :string, autogenerate: false}
  schema "notebooks" do
    field :project, :string, default: ""
    field :title, :string, default: ""
    field :created_at, :naive_datetime
    field :updated_at, :naive_datetime
    # Set by the Go server's soft delete; a deleted notebook is hidden everywhere.
    field :deleted_at, :naive_datetime
  end
end
