defmodule CouncilHubUi.Council.NotebookEntry do
  use Ecto.Schema

  @primary_key {:id, :string, autogenerate: false}
  schema "notebook_entries" do
    field :notebook_id, :string
    field :position, :integer
    field :kind, :string, default: "prose"
    field :ref_id, :string, default: ""
    field :prose, :string, default: ""
    field :status, :string, default: "open"
    field :created_at, :naive_datetime
  end
end
