defmodule CouncilHubUi.CouncilFixtures do
  @moduledoc "Test helpers for creating council rooms and messages."

  alias CouncilHubUi.Repo

  def create_room(attrs \\ %{}) do
    defaults = %{
      id: "test-room-#{System.unique_integer([:positive])}",
      description: "A test room",
      status: "active",
      project: "",
      tech_stack: "",
      tags: "",
      system_prompt: "",
      related_rooms: "",
      created_at: NaiveDateTime.utc_now(),
      updated_at: NaiveDateTime.utc_now()
    }

    merged = Map.merge(defaults, attrs)

    Repo.insert_all("rooms", [merged])
    Repo.get(CouncilHubUi.Council.Room, merged.id)
  end

  def create_message(attrs \\ %{}) do
    # Generate monotonically increasing IDs so get_messages_since ordering works in tests.
    # Format: "019d0000-0000-7000-8000-xxxxxxxxxxxx" where the last segment is a hex counter.
    counter = System.unique_integer([:positive, :monotonic]) |> rem(281_474_976_710_655)
    last_part = counter |> Integer.to_string(16) |> String.pad_leading(12, "0")
    ordered_id = "019d0000-0000-7000-8000-#{last_part}"

    defaults = %{
      id: ordered_id,
      room_id: "test-room",
      author: "Claude",
      content: "Test message",
      message_type: "message",
      is_summary: 0,
      reply_to: "",
      pinned: 0,
      timestamp: NaiveDateTime.utc_now()
    }

    merged = Map.merge(defaults, attrs)
    # SQLite stores booleans as integers
    merged =
      Map.update!(merged, :is_summary, fn
        true -> 1
        false -> 0
        v -> v
      end)

    merged =
      Map.update!(merged, :pinned, fn
        true -> 1
        false -> 0
        v -> v
      end)

    {1, nil} = Repo.insert_all("messages", [merged])

    # Return the last inserted message
    import Ecto.Query
    Repo.one(from m in CouncilHubUi.Council.Message, order_by: [desc: m.id], limit: 1)
  end
end
