defmodule CouncilHubUi.HealthStatsTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.{HealthStats, Repo}
  import CouncilHubUi.CouncilFixtures

  # The real vec0 shadow table is created by the Go server; recreate a plain
  # stand-in (one row per stored vector) so the count path is exercised without
  # the sqlite-vec extension — exactly the read the UI does in production.
  defp seed_vec_shadow(message_ids) do
    Ecto.Adapters.SQL.query!(
      Repo,
      "CREATE TABLE IF NOT EXISTS message_vectors_rowids (rowid INTEGER PRIMARY KEY, id TEXT)",
      []
    )

    for id <- message_ids do
      Ecto.Adapters.SQL.query!(Repo, "INSERT INTO message_vectors_rowids (id) VALUES (?)", [id])
    end
  end

  describe "db_stats embedded coverage" do
    test "reads the vec0 shadow table and computes coverage" do
      create_room(%{id: "hs-room"})
      ids = for _ <- 1..4, do: create_message(%{room_id: "hs-room"}).id
      seed_vec_shadow(Enum.take(ids, 3))

      stats = HealthStats.db_stats()

      assert stats.embedded == 3
      assert stats.embeddable == 4
      assert stats.coverage_pct == 75
    end

    # Superseded revisions and retractions are never embedded by design, and a
    # vector left from before an edit or retraction isn't searchable, so coverage
    # counts live heads only on both sides. The plain message total is unchanged.
    test "coverage counts only live messages and their vectors" do
      create_room(%{id: "hs-live"})
      live = create_message(%{room_id: "hs-live"})
      _unembedded_live = create_message(%{room_id: "hs-live"})
      revised = create_message(%{room_id: "hs-live", revised: 1})
      retracted = create_message(%{room_id: "hs-live", retracted_at: NaiveDateTime.utc_now()})
      seed_vec_shadow([live.id, revised.id, retracted.id])

      stats = HealthStats.db_stats()

      assert stats.message_count == 4
      assert stats.embedded == 1
      assert stats.embeddable == 2
      assert stats.coverage_pct == 50
    end

    test "no shadow table → embedded nil (semantic search not enabled)" do
      Ecto.Adapters.SQL.query!(Repo, "DROP TABLE IF EXISTS message_vectors_rowids", [])
      create_room(%{id: "hs-room2"})
      create_message(%{room_id: "hs-room2"})

      stats = HealthStats.db_stats()

      assert is_nil(stats.embedded)
      assert is_nil(stats.coverage_pct)
    end
  end
end
