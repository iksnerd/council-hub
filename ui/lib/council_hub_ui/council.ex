defmodule CouncilHubUi.Council do
  @moduledoc """
  Read-only context for querying council rooms and messages.
  Thin facade — delegates to focused sub-modules. The Go MCP server owns writes.

  cluster.ex uses `apply(Council, func, args)` for RPC fan-out, so all public
  function names must remain stable here.
  """

  # -- Rooms --

  defdelegate list_rooms(), to: CouncilHubUi.CouncilRooms
  defdelegate get_room(id), to: CouncilHubUi.CouncilRooms
  defdelegate get_room_with_messages(room_id), to: CouncilHubUi.CouncilRooms
  defdelegate list_rooms_filtered(params), to: CouncilHubUi.CouncilRooms
  defdelegate room_stats(room_id), to: CouncilHubUi.CouncilRooms
  defdelegate room_participants(room_id), to: CouncilHubUi.CouncilRooms
  defdelegate room_participants_with_counts(room_id), to: CouncilHubUi.CouncilRooms

  # -- Messages (explicit delegation to support default arguments) --

  def list_messages_for_room(room_id, type_filter \\ "all"),
    do: CouncilHubUi.CouncilMessages.list_messages_for_room(room_id, type_filter)

  def get_messages_since(room_id, last_id, type_filter \\ "all"),
    do: CouncilHubUi.CouncilMessages.get_messages_since(room_id, last_id, type_filter)

  def search_messages_in_room(room_id, query, type_filter \\ "all"),
    do: CouncilHubUi.CouncilMessages.search_messages_in_room(room_id, query, type_filter)

  defdelegate search_messages(params), to: CouncilHubUi.CouncilMessages
  defdelegate get_messages_by_ids(ids), to: CouncilHubUi.CouncilMessages
  defdelegate get_recent_messages(room_id, limit), to: CouncilHubUi.CouncilMessages

  def get_mentions(author, limit \\ 20),
    do: CouncilHubUi.CouncilMessages.get_mentions(author, limit)

  # -- Bulk stats --

  defdelegate all_room_participant_counts(), to: CouncilHubUi.BulkStats
  defdelegate all_room_message_counts(), to: CouncilHubUi.BulkStats
  defdelegate all_room_synthesis_flags(), to: CouncilHubUi.BulkStats
  defdelegate all_room_latest_message_ids(), to: CouncilHubUi.BulkStats
  defdelegate all_room_key_type_counts(), to: CouncilHubUi.BulkStats
  defdelegate all_room_full_type_counts(), to: CouncilHubUi.BulkStats
  defdelegate all_room_time_ranges(), to: CouncilHubUi.BulkStats
  defdelegate latest_room_update(), to: CouncilHubUi.BulkStats

  # -- Digest --

  defdelegate get_project_digest(project, since_str), to: CouncilHubUi.CouncilDigest

  # -- Notebook --

  defdelegate notebook_entries(params), to: CouncilHubUi.CouncilNotebook
  defdelegate list_projects(), to: CouncilHubUi.CouncilNotebook

  def list_notebooks(project \\ ""), do: CouncilHubUi.CouncilNotebook.list_notebooks(project)

  defdelegate get_notebook(id), to: CouncilHubUi.CouncilNotebook
  defdelegate outline_entries(notebook_id), to: CouncilHubUi.CouncilNotebook

  # -- Skills registry (E3) --

  defdelegate list_skills(opts), to: CouncilHubUi.CouncilSkills
  defdelegate get_skill(name), to: CouncilHubUi.CouncilSkills
  defdelegate list_skill_projects(), to: CouncilHubUi.CouncilSkills

  # -- Format --

  defdelegate format_transcript(room, messages), to: CouncilHubUi.CouncilFormat
end
