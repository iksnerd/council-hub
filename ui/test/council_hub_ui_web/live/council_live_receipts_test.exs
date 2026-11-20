defmodule CouncilHubUiWeb.CouncilLiveReceiptsTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest
  import CouncilHubUi.CouncilFixtures

  alias CouncilHubUi.Repo

  defp sql(q), do: Ecto.Adapters.SQL.query!(Repo, q, [])

  test "a receipt written after the room is open flips a chip from pending to seen", %{conn: conn} do
    sql(
      "CREATE TABLE IF NOT EXISTS mention_acks (agent TEXT NOT NULL, message_id TEXT NOT NULL, acked_at DATETIME DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (agent, message_id))"
    )

    room = create_room(%{id: "rc-live-room"})
    msg = create_message(%{room_id: room.id, content: "ping", mentions: "claude"})

    {:ok, view, html} = live(conn, "/rooms/rc-live-room")
    assert html =~ "pending"

    sql("INSERT INTO mention_acks VALUES ('claude', '#{msg.id}', '2026-11-20 10:00:00')")
    send(view.pid, :poll_messages)
    html = render(view)
    assert html =~ "seen "
    refute html =~ "pending"
  end

  test "the Agents panel shows agents from agent_seen on the dashboard", %{conn: conn} do
    sql(
      "CREATE TABLE IF NOT EXISTS agent_seen (agent TEXT PRIMARY KEY, last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP, last_tool TEXT DEFAULT '')"
    )

    now = NaiveDateTime.utc_now() |> NaiveDateTime.to_string() |> String.slice(0, 19)
    sql("INSERT OR REPLACE INTO agent_seen VALUES ('panel-agent', '#{now}', 'read_room')")

    {:ok, _view, html} = live(conn, "/")
    assert html =~ "panel-agent"
  end
end
