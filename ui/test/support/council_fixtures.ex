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
    defaults = %{
      room_id: "test-room",
      author: "Claude",
      content: "Test message",
      message_type: "message",
      is_summary: 0,
      reply_to: 0,
      timestamp: NaiveDateTime.utc_now()
    }

    merged = Map.merge(defaults, attrs)
    # SQLite stores booleans as integers
    merged = Map.update!(merged, :is_summary, fn
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
