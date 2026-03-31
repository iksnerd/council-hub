defmodule CouncilHubUi.Council.Message do
  use Ecto.Schema

  alias CouncilHubUi.Council.Room

  schema "messages" do
    belongs_to :room, Room, type: :string
    field :author, :string
    field :content, :string
    field :message_type, :string, default: "message"
    field :is_summary, :boolean, default: false
    field :reply_to, :integer, default: 0
    field :pinned, :boolean, default: false
    field :timestamp, :naive_datetime
  end
end
