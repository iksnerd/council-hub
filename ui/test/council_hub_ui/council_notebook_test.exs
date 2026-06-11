defmodule CouncilHubUi.CouncilNotebookTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.Council
  import CouncilHubUi.CouncilFixtures

  defp seed_project do
    room_a = create_room(%{id: "nb-room-a", project: "nb-proj", repo: "alice/widgets"})
    room_b = create_room(%{id: "nb-room-b", project: "nb-proj"})
    other = create_room(%{id: "nb-other", project: "other-proj"})

    m1 =
      create_message(%{
        room_id: room_a.id,
        author: "claude",
        content: "use SQLite",
        message_type: "decision"
      })

    create_message(%{room_id: room_a.id, content: "plain chatter"})

    m2 =
      create_message(%{
        room_id: room_b.id,
        author: "gemini",
        content: "parser shipped {sha:abc1234}",
        message_type: "action"
      })

    m3 =
      create_message(%{
        room_id: room_a.id,
        author: "claude",
        content: "compiled findings",
        message_type: "synthesis"
      })

    create_message(%{room_id: other.id, content: "other decision", message_type: "decision"})

    {m1, m2, m3}
  end

  describe "notebook_entries/1" do
    test "weaves typed messages from all project rooms chronologically" do
      {m1, m2, m3} = seed_project()

      entries = Council.notebook_entries(%{"project" => "nb-proj"})

      assert Enum.map(entries, & &1.id) == [m1.id, m2.id, m3.id]
      assert Enum.map(entries, & &1.room_id) == ["nb-room-a", "nb-room-b", "nb-room-a"]
    end

    test "excludes untyped messages by default and carries the owning room's repo" do
      seed_project()

      entries = Council.notebook_entries(%{"project" => "nb-proj"})

      refute Enum.any?(entries, &(&1.content == "plain chatter"))
      assert Enum.at(entries, 0).repo == "alice/widgets"
      assert Enum.at(entries, 1).repo == ""
    end

    test "excludes other projects" do
      seed_project()

      entries = Council.notebook_entries(%{"project" => "nb-proj"})
      refute Enum.any?(entries, &(&1.room_id == "nb-other"))
    end

    test "filters by types CSV" do
      seed_project()

      entries = Council.notebook_entries(%{"project" => "nb-proj", "types" => "action"})
      assert [%{message_type: "action"}] = entries
    end

    test "after_id returns only newer entries" do
      {m1, m2, m3} = seed_project()

      entries = Council.notebook_entries(%{"project" => "nb-proj", "after_id" => m1.id})
      assert Enum.map(entries, & &1.id) == [m2.id, m3.id]
    end

    test "limit keeps the most recent entries, still chronological" do
      {_m1, m2, m3} = seed_project()

      entries = Council.notebook_entries(%{"project" => "nb-proj", "limit" => 2})
      assert Enum.map(entries, & &1.id) == [m2.id, m3.id]
    end

    test "since and until bound the window" do
      seed_project()

      assert Council.notebook_entries(%{
               "project" => "nb-proj",
               "since" => "2099-01-01T00:00:00"
             }) == []

      entries =
        Council.notebook_entries(%{"project" => "nb-proj", "until" => "2099-01-01T00:00:00"})

      assert length(entries) == 3
    end

    test "empty project returns no entries" do
      seed_project()
      assert Council.notebook_entries(%{"project" => ""}) == []
    end
  end

  describe "list_projects/0" do
    test "returns distinct non-empty projects alphabetically" do
      seed_project()
      create_room(%{id: "nb-no-proj", project: ""})

      assert Council.list_projects() == ["nb-proj", "other-proj"]
    end
  end
end
