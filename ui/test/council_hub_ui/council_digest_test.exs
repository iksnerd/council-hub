defmodule CouncilHubUi.CouncilDigestTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.Council
  import CouncilHubUi.CouncilFixtures

  @yesterday NaiveDateTime.utc_now() |> NaiveDateTime.add(-86400, :second)

  describe "get_project_digest/2" do
    test "returns rooms with new messages and an excerpt" do
      room = create_room(%{id: "dg-basic", project: "dg-proj"})
      create_message(%{room_id: room.id, content: "fresh news here"})
      quiet = create_room(%{id: "dg-quiet", project: "dg-proj"})
      _ = quiet

      [entry] = Council.get_project_digest("dg-proj", NaiveDateTime.to_iso8601(@yesterday))

      assert entry.room_id == "dg-basic"
      assert entry.new_message_count == 1
      assert entry.latest_message_excerpt =~ "fresh news here"
    end

    test "counts exclude revised (superseded) nodes — no +1 per edit" do
      room = create_room(%{id: "dg-revised", project: "dg-proj"})
      v1 = create_message(%{room_id: room.id, content: "v1", revised: 1})
      _v2 = create_message(%{room_id: room.id, content: "v2 final", revises: v1.id})

      [entry] = Council.get_project_digest("dg-proj", NaiveDateTime.to_iso8601(@yesterday))

      assert entry.new_message_count == 1
      assert entry.latest_message_excerpt =~ "v2 final"
    end

    test "counts exclude retracted tombstones" do
      room = create_room(%{id: "dg-count-retract", project: "dg-proj"})
      create_message(%{room_id: room.id, content: "kept"})

      create_message(%{
        room_id: room.id,
        content: "withdrawn",
        retracted_at: NaiveDateTime.utc_now(),
        retracted_by: "claude"
      })

      [entry] = Council.get_project_digest("dg-proj", NaiveDateTime.to_iso8601(@yesterday))

      assert entry.new_message_count == 1
    end

    test "a retracted latest renders as a tombstone, never its content" do
      room = create_room(%{id: "dg-retract", project: "dg-proj"})
      create_message(%{room_id: room.id, content: "older live message"})

      create_message(%{
        room_id: room.id,
        content: "super secret withdrawn text",
        retracted_at: NaiveDateTime.utc_now(),
        retracted_by: "claude"
      })

      [entry] = Council.get_project_digest("dg-proj", NaiveDateTime.to_iso8601(@yesterday))

      assert entry.latest_message_excerpt == "[retracted by claude]"
      refute entry.latest_message_excerpt =~ "secret"
    end

    test "an anonymously retracted latest renders the bare tombstone" do
      room = create_room(%{id: "dg-retract-anon", project: "dg-proj"})
      create_message(%{room_id: room.id, content: "live"})

      create_message(%{
        room_id: room.id,
        content: "withdrawn",
        retracted_at: NaiveDateTime.utc_now()
      })

      [entry] = Council.get_project_digest("dg-proj", NaiveDateTime.to_iso8601(@yesterday))

      assert entry.latest_message_excerpt == "[retracted]"
    end

    test "the latest excerpt collapses to the head revision, not a stale prior version" do
      room = create_room(%{id: "dg-head", project: "dg-proj"})
      v1 = create_message(%{room_id: room.id, content: "stale draft", revised: 1})
      _v2 = create_message(%{room_id: room.id, content: "current head", revises: v1.id})

      [entry] = Council.get_project_digest("dg-proj", NaiveDateTime.to_iso8601(@yesterday))

      assert entry.latest_message_excerpt =~ "current head"
      refute entry.latest_message_excerpt =~ "stale draft"
    end

    test "date-only since works (lenient parse instead of the silent 24h fallback)" do
      room = create_room(%{id: "dg-date-only", project: "dg-proj"})
      create_message(%{room_id: room.id, content: "in range"})

      yesterday_date = @yesterday |> NaiveDateTime.to_date() |> Date.to_iso8601()
      [entry] = Council.get_project_digest("dg-proj", yesterday_date)
      assert entry.new_message_count == 1

      tomorrow_date =
        NaiveDateTime.utc_now()
        |> NaiveDateTime.add(86400, :second)
        |> NaiveDateTime.to_date()
        |> Date.to_iso8601()

      assert Council.get_project_digest("dg-proj", tomorrow_date) == []
    end

    test "empty since falls back to the last 24h" do
      room = create_room(%{id: "dg-empty-since", project: "dg-proj"})
      create_message(%{room_id: room.id, content: "recent"})

      create_message(%{
        room_id: room.id,
        content: "ancient",
        timestamp: NaiveDateTime.add(NaiveDateTime.utc_now(), -172_800, :second)
      })

      [entry] = Council.get_project_digest("dg-proj", "")
      assert entry.new_message_count == 1
    end

    test "scopes to the given project" do
      r1 = create_room(%{id: "dg-in", project: "dg-proj"})
      r2 = create_room(%{id: "dg-out", project: "other"})
      create_message(%{room_id: r1.id, content: "mine"})
      create_message(%{room_id: r2.id, content: "theirs"})

      entries = Council.get_project_digest("dg-proj", NaiveDateTime.to_iso8601(@yesterday))
      assert Enum.map(entries, & &1.room_id) == ["dg-in"]
    end
  end
end
