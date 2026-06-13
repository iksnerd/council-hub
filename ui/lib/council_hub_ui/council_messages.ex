defmodule CouncilHubUi.CouncilMessages do
  @moduledoc "Read-only message queries. Called via CouncilHubUi.Council facade."

  import Ecto.Query
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.{Room, Message}

  def list_messages_for_room(room_id, type_filter \\ "all") do
    base = from m in Message, where: m.room_id == ^room_id

    base =
      if type_filter != "all",
        do: from(m in base, where: m.message_type == ^type_filter or m.is_summary == true),
        else: base

    Repo.all(from m in base, order_by: [desc: m.pinned, asc: m.id])
    |> annotate_superseded_by()
  end

  # Sets the virtual :superseded_by on each message that a later message replaces,
  # so the dashboard can render the reverse of the `supersedes` link (the backlink).
  defp annotate_superseded_by(messages) do
    by =
      for m <- messages, m.supersedes not in [nil, ""], into: %{}, do: {m.supersedes, m.id}

    Enum.map(messages, fn m -> %{m | superseded_by: Map.get(by, m.id, "")} end)
  end

  def get_messages_since(room_id, last_id, type_filter \\ "all") do
    base = from m in Message, where: m.room_id == ^room_id and m.id > ^last_id

    base =
      if type_filter != "all",
        do: from(m in base, where: m.message_type == ^type_filter or m.is_summary == true),
        else: base

    Repo.all(from m in base, order_by: [asc: m.id])
  end

  def search_messages_in_room(room_id, query, type_filter \\ "all")

  def search_messages_in_room(_room_id, query, _type_filter) when query in [nil, ""], do: []

  def search_messages_in_room(room_id, query, type_filter) do
    q = "%#{String.downcase(query)}%"

    base =
      from m in Message,
        where:
          m.room_id == ^room_id and
            fragment("lower(?) LIKE ?", m.content, ^q)

    base =
      if type_filter != "all",
        do: from(m in base, where: m.message_type == ^type_filter or m.is_summary == true),
        else: base

    Repo.all(from m in base, order_by: [asc: m.id], limit: 50)
  end

  @doc """
  Search messages with optional filters. Mirrors Go SearchMessages().
  Accepts a map with optional keys: query, author, message_type, room_id, project, limit.
  """
  def search_messages(params) when is_map(params) do
    limit = Map.get(params, "limit", 20)

    base = from(m in Message, as: :msg)

    base =
      case Map.get(params, "query") do
        nil ->
          base

        "" ->
          base

        q ->
          words = String.split(q, ~r/\s+/, trim: true)

          Enum.reduce(words, base, fn word, acc ->
            pattern = "%#{word}%"
            from([msg: m] in acc, where: like(m.content, ^pattern))
          end)
      end

    base =
      case Map.get(params, "author") do
        nil -> base
        "" -> base
        a -> from([msg: m] in base, where: m.author == ^a)
      end

    base =
      case Map.get(params, "message_type") do
        nil -> base
        "" -> base
        t -> from([msg: m] in base, where: m.message_type == ^t)
      end

    base =
      case Map.get(params, "room_id") do
        nil -> base
        "" -> base
        r -> from([msg: m] in base, where: m.room_id == ^r)
      end

    base =
      case Map.get(params, "room_ids") do
        nil ->
          base

        "" ->
          base

        ids_str ->
          ids = ids_str |> String.split(",", trim: true) |> Enum.map(&String.trim/1)
          from([msg: m] in base, where: m.room_id in ^ids)
      end

    base =
      case Map.get(params, "project") do
        nil ->
          base

        "" ->
          base

        p ->
          from([msg: m] in base,
            join: r in Room,
            on: m.room_id == r.id,
            where: r.project == ^p
          )
      end

    base =
      case Map.get(params, "since") do
        nil -> base
        "" -> base
        since -> from([msg: m] in base, where: m.timestamp >= ^since)
      end

    base =
      case Map.get(params, "until") do
        nil -> base
        "" -> base
        until_val -> from([msg: m] in base, where: m.timestamp <= ^until_val)
      end

    Repo.all(
      from [msg: m] in base,
        order_by: [desc: m.timestamp],
        limit: ^limit
    )
  end

  @doc "Fetch messages by a list of IDs"
  def get_messages_by_ids(ids) when is_list(ids) do
    Repo.all(from m in Message, where: m.id in ^ids, order_by: [asc: m.timestamp])
  end

  @doc "Fetch recent messages for a room with a limit"
  def get_recent_messages(room_id, limit) do
    Repo.all(
      from m in Message, where: m.room_id == ^room_id, order_by: [desc: m.id], limit: ^limit
    )
  end

  def get_mentions(author, limit \\ 20) do
    pattern = "%#{author}%"

    Repo.all(
      from m in Message,
        where: like(m.mentions, ^pattern) and m.mentions != "",
        order_by: [desc: m.timestamp],
        limit: ^limit,
        select: %{
          id: m.id,
          room_id: m.room_id,
          author: m.author,
          content: m.content,
          timestamp: m.timestamp
        }
    )
  end
end
