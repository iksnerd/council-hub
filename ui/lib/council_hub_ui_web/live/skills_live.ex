defmodule CouncilHubUiWeb.SkillsLive do
  @moduledoc """
  Skills registry browser: a read-only view of the Go server's `skills` table —
  the methodology registry (E3). The UI twin of the `query_skills_registry` MCP
  tool (same project-plus-global rule, same substring/tag filters). Read-only;
  the write surface is the `register_skill` MCP tool.
  """
  use CouncilHubUiWeb, :live_view

  alias CouncilHubUi.Council
  import CouncilHubUiWeb.CouncilHelpers, only: [render_markdown: 1, format_timestamp: 1]

  @refresh_interval 5_000

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket), do: Process.send_after(self(), :refresh, @refresh_interval)
    {:ok, assign(socket, page_title: "Skills")}
  end

  @impl true
  def handle_params(params, _uri, socket) do
    {:noreply,
     socket
     |> assign(
       project: Map.get(params, "project", ""),
       query: Map.get(params, "query", ""),
       tag: Map.get(params, "tag", "")
     )
     |> load_skills()}
  end

  @impl true
  def handle_event("filter", params, socket) do
    {:noreply,
     push_patch(socket,
       to:
         ~p"/skills?#{skills_params(Map.get(params, "project", ""), Map.get(params, "query", ""), Map.get(params, "tag", ""))}"
     )}
  end

  @impl true
  def handle_info(:refresh, socket) do
    Process.send_after(self(), :refresh, @refresh_interval)
    {:noreply, load_skills(socket)}
  end

  defp load_skills(socket) do
    %{project: project, query: query, tag: tag} = socket.assigns

    assign(socket,
      skills: Council.list_skills(%{project: project, query: query, tag: tag}),
      projects: Council.list_skill_projects()
    )
  end

  # Drop empty filters so the URL stays clean and shareable.
  defp skills_params(project, query, tag) do
    [{"project", project}, {"query", query}, {"tag", tag}]
    |> Enum.reject(fn {_k, v} -> v in [nil, ""] end)
    |> Map.new()
  end

  def scope_label(""), do: "global"
  def scope_label(project), do: project
end
