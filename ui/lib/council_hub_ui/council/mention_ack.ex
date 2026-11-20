defmodule CouncilHubUi.Council.MentionAck do
  use Ecto.Schema

  # Read-only mirror of the Go server's mention_acks table: one row per
  # (agent, message) the first time get_mentions(ack=true) returned that message
  # to that agent. `agent` is the trimmed, lowercased name. Go owns writes, and
  # the table is absent on a database created by an older Go server.
  @primary_key false
  schema "mention_acks" do
    field :agent, :string
    field :message_id, :string
    field :acked_at, :naive_datetime
  end
end
