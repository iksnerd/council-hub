defmodule CouncilHubUi.HealthStats do
  @moduledoc """
  Read-only DB health metrics for the Status page. All queries run against the
  shared SQLite file via the Phoenix read connection — no writes.
  """
  import Ecto.Query

  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.{Room, Message}

  @doc """
  Returns a map of database-level health metrics:
  room/message counts, last activity timestamp, private-room count, and
  semantic-search embedding coverage.
  """
  def db_stats do
    message_count = Repo.aggregate(Message, :count)
    embedded = embedded_count()

    %{
      room_count: Repo.aggregate(Room, :count),
      message_count: message_count,
      private_rooms: Repo.aggregate(from(r in Room, where: r.visibility == "private"), :count),
      last_message_at: Repo.one(from m in Message, select: max(m.timestamp)),
      embedded: embedded,
      coverage_pct: coverage_pct(embedded, message_count)
    }
  end

  defp coverage_pct(embedded, total)
       when is_integer(embedded) and is_integer(total) and total > 0,
       do: round(embedded * 100 / total)

  defp coverage_pct(_embedded, _total), do: nil

  # message_vectors is a sqlite-vec *virtual* table (vec0) owned by the Go server.
  # The Phoenix read connection has no vec0 module loaded (the Go cgo driver
  # statically links sqlite-vec; ecto_sqlite3 doesn't), so `SELECT FROM
  # message_vectors` errors and the count looks unavailable even when semantic
  # search is fully enabled and populated. Read the vec0 shadow table
  # `message_vectors_rowids` instead — a plain table (one row per stored vector)
  # that any connection can read, so the count is accurate without the extension.
  # A genuine failure (table absent → semantic search not enabled) still → nil.
  defp embedded_count do
    case Ecto.Adapters.SQL.query(Repo, "SELECT count(*) FROM message_vectors_rowids", []) do
      {:ok, %{rows: [[n]]}} -> n
      _ -> nil
    end
  rescue
    _ -> nil
  end
end
