defmodule CouncilHubUi.GoTimestampFormatTest do
  @moduledoc """
  The Go server stores message timestamps as 'YYYY-MM-DD HH:MM:SS' (SQLite
  CURRENT_TIMESTAMP style, a space separator). SQLite compares them as text, so a
  bound bound as ISO 'T' text ('...T21:00:00') sorts after every same-date row
  (0x54 > 0x20). Fixtures write through Ecto, which never exercised this; these
  tests insert Go-format rows with raw SQL.
  """
  use CouncilHubUi.DataCase

  alias CouncilHubUi.{Council, CouncilMessages, CouncilNotebook, Repo}
  import CouncilHubUi.CouncilFixtures

  defp go_message(room_id, id, ts, type \\ "decision") do
    Repo.query!(
      "INSERT INTO messages (id, room_id, author, content, message_type, is_summary, reply_to, pinned, timestamp) " <>
        "VALUES (?, ?, 'a', ?, ?, 0, '', 0, ?)",
      [id, room_id, "msg #{id}", type, ts]
    )
  end

  setup do
    room = create_room(%{id: "gotsroom", project: "gots-proj"})
    go_message(room.id, "019d0000-0000-7000-8000-00000000a001", "2026-10-19 09:00:00")
    go_message(room.id, "019d0000-0000-7000-8000-00000000a002", "2026-10-19 21:00:00")
    go_message(room.id, "019d0000-0000-7000-8000-00000000a003", "2026-10-19 23:30:00")
    :ok
  end

  defp ids(msgs), do: msgs |> Enum.map(& &1.id) |> Enum.map(&String.last/1) |> Enum.sort()

  describe "digest since" do
    test "same-date T bound counts messages after it" do
      [entry] = Council.get_project_digest("gots-proj", "2026-10-19T00:00:00")
      assert entry.new_message_count == 3
    end

    test "bound mid-day is strict and excludes the row at the boundary" do
      [entry] = Council.get_project_digest("gots-proj", "2026-10-19T09:00:00")
      assert entry.new_message_count == 2
      [entry] = Council.get_project_digest("gots-proj", "2026-10-19T21:00:00")
      assert entry.new_message_count == 1
    end

    test "space-separated bound agrees" do
      [entry] = Council.get_project_digest("gots-proj", "2026-10-19 09:00:00")
      assert entry.new_message_count == 2
    end
  end

  describe "search_messages since/until" do
    test "since on the same date" do
      r =
        CouncilMessages.search_messages(%{
          "room_id" => "gotsroom",
          "since" => "2026-10-19T00:00:00"
        })

      assert length(r) == 3

      r =
        CouncilMessages.search_messages(%{
          "room_id" => "gotsroom",
          "since" => "2026-10-19T21:00:00"
        })

      assert ids(r) == ["2", "3"]
    end

    test "until on the same date, inclusive at the boundary" do
      r =
        CouncilMessages.search_messages(%{
          "room_id" => "gotsroom",
          "until" => "2026-10-19T21:00:00"
        })

      assert ids(r) == ["1", "2"]

      r =
        CouncilMessages.search_messages(%{
          "room_id" => "gotsroom",
          "until" => "2026-10-19T08:59:59"
        })

      assert r == []
    end

    test "date-only until includes the whole day" do
      r = CouncilMessages.search_messages(%{"room_id" => "gotsroom", "until" => "2026-10-19"})
      assert length(r) == 3
    end
  end

  describe "notebook since/until" do
    test "since and until on the same date" do
      p = %{"project" => "gots-proj"}

      assert length(CouncilNotebook.notebook_entries(Map.put(p, "since", "2026-10-19T00:00:00"))) ==
               3

      assert length(CouncilNotebook.notebook_entries(Map.put(p, "since", "2026-10-19T21:00:00"))) ==
               2

      assert length(CouncilNotebook.notebook_entries(Map.put(p, "until", "2026-10-19T21:00:00"))) ==
               2

      assert CouncilNotebook.notebook_entries(Map.put(p, "until", "2026-10-19T08:00:00")) == []
    end
  end
end
