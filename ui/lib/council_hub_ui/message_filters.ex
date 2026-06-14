defmodule CouncilHubUi.MessageFilters do
  @moduledoc """
  Composable Ecto filters for the append-only message ledger — the single place the
  "live node" rules live on the read side (the Elixir mirror of the Go
  `headClause`/`liveClause` helpers). Edits are append-only, so non-head revisions
  (`revised`) and retracted tombstones must be filtered from content reads. Pipe a
  query through these instead of re-writing the predicate at each call site.
  """
  import Ecto.Query

  @doc """
  Collapse to head revisions — hide versions superseded by a later edit. Retracted
  nodes pass this filter (they still render as tombstones in the room view).
  """
  def head_revisions(query), do: where(query, [m], m.revised == false)

  @doc """
  Head revisions excluding retracted tombstones — for reads that surface content as a
  hit (search, the notebook timeline) rather than as a visible tombstone.
  """
  def live_messages(query), do: where(query, [m], m.revised == false and is_nil(m.retracted_at))
end
