defmodule CouncilHubUi.Council.Room do
  use Ecto.Schema

  @primary_key {:id, :string, autogenerate: false}
  schema "rooms" do
    field :description, :string
    field :status, :string, default: "active"
    field :project, :string, default: ""
    field :tech_stack, :string, default: ""
    field :tags, :string, default: ""
    field :system_prompt, :string, default: ""
    field :related_rooms, :string, default: ""
    field :created_at, :naive_datetime
    field :updated_at, :naive_datetime
  end
end
