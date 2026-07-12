defmodule CouncilHubUi.CouncilNotebook do
  @moduledoc """
  Read-only project notebook queries: typed messages from every room in a
  project woven into one chronological timeline. Mirrors the Go server's
  GetNotebookEntries (notebook.go) — UUIDv7 message IDs sort lexicographically
  by creation time, so ordering by id merges rooms without a timestamp merge.
  Called via the CouncilHubUi.Council facade (locally and via cluster fan-out).
  """

  import Ecto.Query
  import CouncilHubUi.MessageFilters
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.{Room, Message, Notebook, NotebookEntry}
  alias CouncilHubUi.{MessageAnnotations, Params, Timestamps}

  @default_types ~w(decision plan action synthesis note)
  @default_limit 100
  @max_limit 500

  def default_types, do: @default_types

  @doc """
  Returns notebook entries for a project as maps (message fields + the owning
  room's `repo` for {sha:...} resolution). Accepts a string-keyed params map:
  project (required), types (CSV), since, until, after_id, limit. When limit
  truncates, the most recent entries are kept; output is chronological.
  """
  def notebook_entries(params) when is_map(params) do
    project = Map.get(params, "project", "")

    if project == "" do
      []
    else
      types = parse_types(Map.get(params, "types", ""))
      limit = parse_limit(Map.get(params, "limit"))

      base =
        from(m in Message,
          join: r in Room,
          on: m.room_id == r.id,
          where: r.project == ^project and m.is_summary == false and m.message_type in ^types
        )
        |> live_messages()

      base =
        case Timestamps.parse_since(Map.get(params, "since", "")) do
          nil -> base
          since -> from [m, r] in base, where: m.timestamp >= ^since
        end

      base =
        case Timestamps.parse_until(Map.get(params, "until", "")) do
          nil -> base
          until_ts -> from [m, r] in base, where: m.timestamp <= ^until_ts
        end

      base =
        case Map.get(params, "after_id", "") do
          "" -> base
          after_id -> from [m, r] in base, where: m.id > ^after_id
        end

      Repo.all(
        from [m, r] in base,
          order_by: [desc: m.id],
          limit: ^limit,
          select: %{
            id: m.id,
            room_id: m.room_id,
            author: m.author,
            content: m.content,
            message_type: m.message_type,
            is_summary: m.is_summary,
            reply_to: m.reply_to,
            supersedes: m.supersedes,
            pinned: m.pinned,
            timestamp: m.timestamp,
            repo: r.repo
          }
      )
      |> Enum.reverse()
      |> MessageAnnotations.annotate_superseded_by()
      |> MessageAnnotations.annotate_links()
    end
  end

  @doc """
  Curated notebooks (Phase 2 outlines), most recently updated first, each with
  its entry count. A project query also returns global notebooks (project = "")
  — they belong to every view. Empty project returns everything.
  """
  def list_notebooks(project \\ "") do
    base = from n in Notebook, as: :nb

    base =
      if project == "",
        do: base,
        else: from([nb: n] in base, where: n.project == ^project or n.project == "")

    Repo.all(
      from [nb: n] in base,
        left_join: e in NotebookEntry,
        on: e.notebook_id == n.id,
        group_by: n.id,
        order_by: [desc: n.updated_at],
        select: %{
          id: n.id,
          project: n.project,
          title: n.title,
          updated_at: n.updated_at,
          entry_count: count(e.id)
        }
    )
  end

  def get_notebook(id), do: Repo.get(Notebook, id)

  @doc """
  A notebook outline's entries in order, with refs transcluded live: message
  refs resolve the referenced message (and its room's repo for {sha:...}
  resolution); room refs resolve the room's status, topic, and latest
  decision/action — a notebook of room_refs is a living work list. Mirrors the
  Go server's GetOutline — a dangling ref comes back with ref_found: false
  instead of failing the read.
  """
  def outline_entries(notebook_id) do
    Repo.all(
      from e in NotebookEntry,
        left_join: m in Message,
        on: e.kind == "ref" and m.id == e.ref_id,
        left_join: r in Room,
        on: r.id == m.room_id,
        left_join: rr in Room,
        on: e.kind == "room_ref" and rr.id == e.ref_id,
        where: e.notebook_id == ^notebook_id,
        order_by: [asc: e.position],
        select: %{
          id: e.id,
          position: e.position,
          kind: e.kind,
          ref_id: e.ref_id,
          prose: e.prose,
          status: coalesce(e.status, "open"),
          ref_found: not is_nil(m.id) or not is_nil(rr.id),
          room_id: coalesce(coalesce(m.room_id, rr.id), ""),
          author: coalesce(m.author, ""),
          message_type: coalesce(m.message_type, ""),
          # The fragment's `revised = 0 AND retracted_at IS NULL` hand-writes the
          # live-node predicate (fragments need literal SQL, so MessageFilters
          # can't pipe into a correlated select subquery). MessageFilters is the
          # canonical definition — a change there must be mirrored here.
          content:
            coalesce(
              coalesce(
                m.content,
                fragment(
                  "(SELECT content FROM messages WHERE room_id = ? AND message_type IN ('decision','action') AND revised = 0 AND retracted_at IS NULL ORDER BY id DESC LIMIT 1)",
                  rr.id
                )
              ),
              ""
            ),
          pinned: coalesce(m.pinned, false),
          timestamp: m.timestamp,
          repo: coalesce(coalesce(r.repo, rr.repo), ""),
          room_status: coalesce(rr.status, ""),
          room_topic: coalesce(rr.description, ""),
          retracted_at: m.retracted_at,
          retracted_by: coalesce(m.retracted_by, "")
        }
    )
    |> Enum.map(&resolve_ref_head/1)
  end

  # The main query joins a "ref" entry on its stored ref_id directly — a fixed
  # address. If that message has since been edited (revises appends a new node,
  # flags the old one revised), the join keeps transcluding the stale prior
  # version forever instead of "update the referenced message instead"
  # (the documented outline contract). Walk forward to the current head.
  defp resolve_ref_head(%{kind: "ref", ref_found: true, ref_id: ref_id} = entry) do
    case head_of_revision_chain(ref_id) do
      ^ref_id ->
        entry

      head_id ->
        case Repo.get(Message, head_id) do
          nil ->
            entry

          head ->
            %{
              entry
              | author: head.author,
                message_type: head.message_type,
                content: head.content,
                pinned: head.pinned,
                timestamp: head.timestamp,
                retracted_at: head.retracted_at,
                retracted_by: head.retracted_by || ""
            }
        end
    end
  end

  defp resolve_ref_head(entry), do: entry

  defp head_of_revision_chain(message_id, hops \\ 0)
  defp head_of_revision_chain(message_id, hops) when hops >= 1000, do: message_id

  defp head_of_revision_chain(message_id, hops) do
    # A healthy chain has at most one revision per node, but the pre-transactional
    # update path could fork it (two rows sharing a `revises` value) — Repo.one
    # would raise Ecto.MultipleResultsError on such legacy data. Pick the newest
    # branch deterministically instead (the Go twin's walk also picks one row).
    next =
      Repo.one(
        from m in Message,
          where: m.revises == ^message_id,
          order_by: [desc: m.id],
          limit: 1,
          select: m.id
      )

    case next do
      nil -> message_id
      next_id -> head_of_revision_chain(next_id, hops + 1)
    end
  end

  @doc "Distinct non-empty project names, alphabetical. Feeds the /notebook picker."
  def list_projects do
    Repo.all(
      from r in Room,
        where: r.project != "",
        distinct: true,
        order_by: r.project,
        select: r.project
    )
  end

  defp parse_types(""), do: @default_types
  defp parse_types(nil), do: @default_types

  defp parse_types(csv) when is_binary(csv) do
    case csv
         |> String.split(",", trim: true)
         |> Enum.map(&String.trim/1)
         |> Enum.reject(&(&1 == "")) do
      [] -> @default_types
      types -> types
    end
  end

  defp parse_limit(val), do: Params.clamp_int(val, @default_limit, max: @max_limit)
end
