defmodule CouncilHubUi.Council.MessageLink do
  use Ecto.Schema

  # Read-only mirror of the Go server's message_links table — an explicit typed
  # edge between two messages (refines/contradicts/implements/duplicates/depends-on/
  # relates). The implicit reply/supersedes edges live on the message itself.
  @primary_key {:id, :string, autogenerate: false}
  schema "message_links" do
    field :from_id, :string
    field :to_id, :string
    field :relation, :string
    field :author, :string, default: ""
  end
end
