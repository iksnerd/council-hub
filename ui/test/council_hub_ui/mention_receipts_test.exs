defmodule CouncilHubUi.MentionReceiptsTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.{AgentPresence, MentionReceipts, Repo}
  import CouncilHubUi.CouncilFixtures

  defp sql(q), do: Ecto.Adapters.SQL.query!(Repo, q, [])

  defp create_tables do
    sql(
      "CREATE TABLE IF NOT EXISTS mention_acks (agent TEXT NOT NULL, message_id TEXT NOT NULL, acked_at DATETIME DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (agent, message_id))"
    )

    sql(
      "CREATE TABLE IF NOT EXISTS agent_seen (agent TEXT PRIMARY KEY, last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP, last_tool TEXT DEFAULT '')"
    )
  end

  describe "MentionReceipts" do
    test "annotate sets receipts for messages with mentions, in one pass" do
      create_tables()
      create_room(%{id: "rc-room"})
      m1 = create_message(%{room_id: "rc-room", mentions: "claude, gemini"})
      m2 = create_message(%{room_id: "rc-room", mentions: ""})
      sql("INSERT INTO mention_acks VALUES ('claude', '#{m1.id}', '2026-11-20 10:00:00')")

      [a, b] = MentionReceipts.annotate([m1, m2])
      assert a.mention_receipts == [{"claude", ~N[2026-11-20 10:00:00]}]
      assert b.mention_receipts == nil
    end

    test "chips: seen vs pending, matched the way get_mentions matches" do
      receipts = [{"claude", ~N[2026-11-20 10:00:00]}, {"claude", ~N[2026-11-20 09:00:00]}]

      assert [{"Claude-Code", ~N[2026-11-20 09:00:00]}, {"gemini", nil}] =
               MentionReceipts.chips("Claude-Code, gemini", receipts)

      assert MentionReceipts.chips("gemini", []) == [{"gemini", nil}]
      assert MentionReceipts.chips("gemini", nil) == nil
    end

    test "a missing mention_acks table is :unavailable and leaves messages unannotated" do
      sql("DROP TABLE IF EXISTS mention_acks")
      create_room(%{id: "rc-old"})
      m = create_message(%{room_id: "rc-old", mentions: "claude"})

      assert MentionReceipts.lookup([m.id]) == :unavailable
      assert [%{mention_receipts: nil}] = MentionReceipts.annotate([m])
    end
  end

  describe "AgentPresence.recent" do
    test "lists agents seen in the last 24h, newest first, capped" do
      create_tables()
      sql("DELETE FROM agent_seen")
      now = ~N[2026-11-20 12:00:00]

      sql("INSERT INTO agent_seen VALUES ('old', '2026-11-18 12:00:00', 'list_rooms')")
      sql("INSERT INTO agent_seen VALUES ('mid', '2026-11-20 08:00:00', 'read_room')")
      sql("INSERT INTO agent_seen VALUES ('new', '2026-11-20 11:59:00', 'post_to_room')")
      sql("INSERT INTO agent_seen VALUES ('mid2', '2026-11-20 09:00:00', 'get_digest')")

      assert ["new", "mid2", "mid"] = AgentPresence.recent(12, now) |> Enum.map(& &1.agent)
      assert ["new", "mid2"] = AgentPresence.recent(2, now) |> Enum.map(& &1.agent)
      assert [%{last_tool: "post_to_room"} | _] = AgentPresence.recent(12, now)
    end

    test "a missing agent_seen table reads as empty" do
      sql("DROP TABLE IF EXISTS agent_seen")
      assert AgentPresence.recent() == []
    end
  end
end
