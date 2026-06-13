defmodule CouncilHubUi.Council.Skill do
  @moduledoc """
  Read-only Ecto mirror of the Go `skills` table — the methodology registry
  (E3). Go owns writes via the register_skill MCP tool; the UI only reads.
  """
  use Ecto.Schema

  @primary_key {:name, :string, autogenerate: false}
  schema "skills" do
    field :description, :string, default: ""
    field :when_to_use, :string, default: ""
    field :content, :string, default: ""
    field :project, :string, default: ""
    field :tags, :string, default: ""
    field :source, :string, default: ""
    field :created_at, :naive_datetime
    field :updated_at, :naive_datetime
  end
end
