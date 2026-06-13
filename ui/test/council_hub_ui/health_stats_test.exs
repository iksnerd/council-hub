defmodule CouncilHubUi.HealthStatsTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.{HealthStats, Repo}
  import CouncilHubUi.CouncilFixtures

  # The real vec0 shadow table is created by the Go server; recreate a plain
  # stand-in (one row per stored vector) so the count path is exercised without
  # the sqlite-vec extension — exactly the read the UI does in production.
  defp seed_vec_shadow(n) do
    Ecto.Adapters.SQL.query!(
      Repo,
      "CREATE TABLE IF NOT EXISTS message_vectors_rowids (rowid INTEGER PRIMARY KEY, id TEXT)",
      []
    )

    for i <- 1..n do
      Ecto.Adapters.SQL.query!(Repo, "INSERT INTO message_vectors_rowids (id) VALUES (?)", [
        "m#{i}"
      ])
    end
  end

  describe "db_stats embedded coverage" do
    test "reads the vec0 shadow table and computes coverage" do
      create_room(%{id: "hs-room"})
      for _ <- 1..4, do: create_message(%{room_id: "hs-room"})
      seed_vec_shadow(3)

      stats = HealthStats.db_stats()

      assert stats.embedded == 3
      assert stats.coverage_pct == 75
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
