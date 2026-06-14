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
    # Append-only edits: revises points at the prior version this node replaced; a
    # node with revised? = true is a superseded (non-head) revision, hidden from reads.
    field :revises, :string, default: ""
    field :revised, :boolean, default: false
    # Retraction (the immutable counterpart to deletion): the node survives but
    # renders as a tombstone.
    field :retracted_at, :naive_datetime
    field :retracted_by, :string, default: ""
    field :timestamp, :naive_datetime
    # Derived (not a column): the ID of a later message that supersedes this one,
    # computed over the loaded set so a superseded message shows its backlink.
    field :superseded_by, :string, virtual: true, default: ""
    # Derived: explicit typed links touching this message (from message_links).
    # Each entry is %{relation: ..., other_id: ..., direction: :out | :in}.
    field :links, {:array, :map}, virtual: true, default: []
  end
end
