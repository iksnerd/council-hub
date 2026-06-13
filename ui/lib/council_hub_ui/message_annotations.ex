defmodule CouncilHubUi.MessageAnnotations do
  @moduledoc """
  Shared derived-field annotators for message lists — used by both the room view
  (`CouncilMessages`) and the project notebook (`CouncilNotebook`) so supersedes
  backlinks and the typed link graph surface consistently in both. Works on Message
  structs and plain maps alike (the notebook selects maps).
  """

  import Ecto.Query
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.MessageLink

  @doc "Sets :superseded_by — the ID of a later message that supersedes this one, computed over the given set."
  def annotate_superseded_by(messages) do
    by =
      for m <- messages, Map.get(m, :supersedes) not in [nil, ""], into: %{} do
        {Map.get(m, :supersedes), Map.get(m, :id)}
      end

    Enum.map(messages, fn m -> Map.put(m, :superseded_by, Map.get(by, Map.get(m, :id), "")) end)
  end

  @doc "Sets :links — explicit message_links touching each message — in one batched query."
  def annotate_links([]), do: []

  def annotate_links(messages) do
    ids = Enum.map(messages, &Map.get(&1, :id))
    links = Repo.all(from l in MessageLink, where: l.from_id in ^ids or l.to_id in ^ids)

    out_by = Enum.group_by(links, & &1.from_id)
    in_by = Enum.group_by(links, & &1.to_id)

    Enum.map(messages, fn m ->
      id = Map.get(m, :id)

      edges =
        Enum.map(Map.get(out_by, id, []), fn l ->
          %{relation: l.relation, other_id: l.to_id, direction: :out}
        end) ++
          Enum.map(Map.get(in_by, id, []), fn l ->
            %{relation: l.relation, other_id: l.from_id, direction: :in}
          end)

      Map.put(m, :links, edges)
    end)
  end
end
