defmodule CouncilHubUi.Council.Message do
  use Ecto.Schema

  schema "messages" do
    field :room_id, :string
    field :author, :string
    field :content, :string
    field :message_type, :string, default: "message"
    field :is_summary, :boolean, default: false
    field :timestamp, :naive_datetime
  end
end
