defmodule CouncilHubUi.MentionReceipts do
  @moduledoc """
  Delivery receipts for @mentions, read from the Go server's `mention_acks`
  table. A mention is "seen" when the mentioned agent fetched the message with
  `get_mentions(ack=true)`; that says nothing about a human reading it, and the
  rows are node-local.

  Mixed-version safe: against a database from an older Go server the table does
  not exist. `lookup/1` then returns `:unavailable`, `annotate/1` leaves
  `:mention_receipts` as `nil`, and the UI keeps rendering plain mention tags
  (it never claims "pending" for something it cannot know).
  """

  import Ecto.Query
  require Logger

  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.MentionAck

  @doc """
  One batched query: `{:ok, %{message_id => [{agent, acked_at}]}}` for the ids,
  or `:unavailable` when the table cannot be read.
  """
  def lookup([]), do: {:ok, %{}}

  def lookup(ids) do
    rows = Repo.all(from a in MentionAck, where: a.message_id in ^ids)
    {:ok, Enum.group_by(rows, & &1.message_id, &{&1.agent, &1.acked_at})}
  rescue
    e ->
      Logger.debug("mention_acks unavailable: #{Exception.message(e)}")
      :unavailable
  end

  @doc "Sets :mention_receipts on messages that carry mentions. One query for the whole list."
  def annotate(messages) do
    ids = for m <- messages, Map.get(m, :mentions) not in [nil, ""], do: m.id

    case lookup(ids) do
      {:ok, by_id} ->
        Enum.map(messages, fn m ->
          if m.id in ids,
            do: Map.put(m, :mention_receipts, Map.get(by_id, m.id, [])),
            else: m
        end)

      :unavailable ->
        messages
    end
  end

  @doc """
  Per-mention state for rendering: `[{mention, acked_at | nil}]`. A receipt
  matches a mention the way the Go server matches `get_mentions`: the acking
  agent's key is a case-insensitive substring of the mention. The earliest
  matching receipt wins. `receipts` nil (unknown) yields `nil`.
  """
  def chips(_mentions, nil), do: nil

  def chips(mentions, receipts) do
    for mention <- CouncilHubUiWeb.CouncilHelpers.parse_mentions(mentions) do
      down = String.downcase(mention)

      at =
        receipts
        |> Enum.filter(fn {agent, _} -> agent != "" and String.contains?(down, agent) end)
        |> Enum.map(&elem(&1, 1))
        |> Enum.min(NaiveDateTime, fn -> nil end)

      {mention, at}
    end
  end
end
