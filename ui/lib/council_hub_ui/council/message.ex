defmodule CouncilHubUi.Council.Message do
  use Ecto.Schema

  alias CouncilHubUi.Council.Room

  @primary_key {:id, :string, autogenerate: false}
  schema "messages" do
    belongs_to :room, Room, type: :string
    field :author, :string
    field :content, :string
    field :message_type, :string, default: "message"
    field :is_summary, :boolean, default: false
    field :reply_to, :string, default: ""
    field :pinned, :boolean, default: false
    field :reactions, :string, default: "{}"
    field :mentions, :string, default: ""
    field :supersedes, :string, default: ""
    field :timestamp, :naive_datetime
    # Derived (not a column): the ID of a later message that supersedes this one,
    # computed over the loaded set so a superseded message shows its backlink.
    field :superseded_by, :string, virtual: true, default: ""
  end
end
