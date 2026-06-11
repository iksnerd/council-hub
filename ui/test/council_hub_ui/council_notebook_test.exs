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

    test "includes note entries by default (the human journal type)" do
      seed_project()

      note =
        create_message(%{
          room_id: "nb-room-a",
          author: "human",
          content: "journal observation",
          message_type: "note"
        })

      entries = Council.notebook_entries(%{"project" => "nb-proj"})
      assert List.last(entries).id == note.id
      assert List.last(entries).message_type == "note"
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

  describe "list_notebooks/1" do
    test "returns notebooks with entry counts, optionally scoped to a project" do
      create_notebook(%{id: "nb-one", project: "nb-proj", title: "One"})
      create_notebook(%{id: "nb-two", project: "other-proj"})
      create_notebook_entry(%{notebook_id: "nb-one", prose: "hello"})
      create_notebook_entry(%{notebook_id: "nb-one", prose: "world"})

      all = Council.list_notebooks()
      assert length(all) == 2

      [nb] = Council.list_notebooks("nb-proj")
      assert nb.id == "nb-one"
      assert nb.title == "One"
      assert nb.entry_count == 2
    end
  end

  describe "outline_entries/1" do
    test "returns entries in position order with refs transcluded" do
      {m1, _m2, _m3} = seed_project()
      create_notebook(%{id: "nb-out", project: "nb-proj"})

      create_notebook_entry(%{
        notebook_id: "nb-out",
        position: 1,
        kind: "prose",
        prose: "## Intro"
      })

      create_notebook_entry(%{
        notebook_id: "nb-out",
        position: 2,
        kind: "ref",
        ref_id: m1.id
      })

      [prose, ref] = Council.outline_entries("nb-out")

      assert prose.kind == "prose"
      assert prose.prose == "## Intro"

      assert ref.kind == "ref"
      assert ref.ref_found
      assert ref.room_id == "nb-room-a"
      assert ref.author == "claude"
      assert ref.content == "use SQLite"
      assert ref.repo == "alice/widgets"
    end

    test "dangling refs come back with ref_found: false" do
      create_notebook(%{id: "nb-dangle", project: "nb-proj"})

      create_notebook_entry(%{
        notebook_id: "nb-dangle",
        position: 1,
        kind: "ref",
        ref_id: "ghost-message-id"
      })

      [entry] = Council.outline_entries("nb-dangle")
      refute entry.ref_found
      assert entry.content == ""
    end
  end
end
