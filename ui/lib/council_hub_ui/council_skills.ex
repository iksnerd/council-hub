defmodule CouncilHubUi.CouncilSkills do
  @moduledoc """
  Read-only mirror of the Go server's `skills` table — the methodology registry
  (E3). Go owns writes (register_skill); the UI reads. The UI twin of the
  `query_skills_registry` MCP tool: same project-plus-global rule, same
  substring/tag filters as council/skills.go.
  """
  import Ecto.Query
  alias CouncilHubUi.Repo
  alias CouncilHubUi.Council.Skill

  @doc """
  Registry entries matching the filters, name-ordered. A non-empty project
  returns that project's skills plus global ones (project = "") — global
  methodology belongs to every project's view, the same rule list_notebooks
  uses. query is a case-insensitive substring across name/description/
  when_to_use/content; tag is a whole-token match within the comma-separated
  tags field (bracketed with commas so "go" can't match "golang").
  """
  def list_skills(opts \\ %{}) do
    project = opts |> Map.get(:project, "") |> to_string()
    query = opts |> Map.get(:query, "") |> to_string() |> String.trim()
    tag = opts |> Map.get(:tag, "") |> to_string() |> String.trim()

    base = from(s in Skill, order_by: [asc: s.name])

    base =
      if project == "",
        do: base,
        else: from(s in base, where: s.project == ^project or s.project == "")

    base =
      if query == "" do
        base
      else
        like = "%#{String.downcase(query)}%"

        from s in base,
          where:
            like(fragment("LOWER(?)", s.name), ^like) or
              like(fragment("LOWER(?)", s.description), ^like) or
              like(fragment("LOWER(?)", s.when_to_use), ^like) or
              like(fragment("LOWER(?)", s.content), ^like)
      end

    base =
      if tag == "" do
        base
      else
        tag_like = "%,#{String.downcase(tag)},%"

        from s in base,
          where: like(fragment("',' || REPLACE(LOWER(?), ' ', '') || ','", s.tags), ^tag_like)
      end

    Repo.all(base)
  end

  @doc "A single skill by exact name, or nil."
  def get_skill(name), do: Repo.get(Skill, name)

  @doc "Distinct non-empty skill project names, alphabetical. Feeds the /skills picker."
  def list_skill_projects do
    Repo.all(
      from s in Skill,
        where: s.project != "",
        distinct: true,
        order_by: s.project,
        select: s.project
    )
  end
end
