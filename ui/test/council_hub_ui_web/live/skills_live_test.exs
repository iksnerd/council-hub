defmodule CouncilHubUiWeb.SkillsLiveTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest
  import CouncilHubUi.CouncilFixtures

  defp seed do
    create_skill(%{
      name: "release",
      project: "council-hub",
      tags: "go,release",
      description: "ship a new version",
      when_to_use: "shipping vX.Y.Z",
      content: "## Steps\n1. bump\n2. tag",
      source: ".claude/skills/release"
    })

    create_skill(%{name: "issue-writer", project: "", description: "draft GitHub issues"})
  end

  describe "skills registry page" do
    test "renders registered skills with scope and content", %{conn: conn} do
      seed()

      {:ok, _view, html} = live(conn, "/skills")

      assert html =~ "Skills Registry"
      assert html =~ "release"
      assert html =~ "ship a new version"
      assert html =~ "shipping vX.Y.Z"
      # global skill carries the global badge
      assert html =~ "issue-writer"
      assert html =~ "global"
      # full playbook body is rendered (inside the <details>)
      assert html =~ "Steps"
      assert html =~ ".claude/skills/release"
    end

    test "empty registry shows the register_skill hint", %{conn: conn} do
      {:ok, _view, html} = live(conn, "/skills")
      assert html =~ "No skills registered yet"
      assert html =~ "register_skill"
    end

    test "query filter narrows the list via the URL", %{conn: conn} do
      seed()

      {:ok, _view, html} = live(conn, "/skills?query=issue")

      assert html =~ "issue-writer"
      refute html =~ "ship a new version"
    end

    test "project filter includes global skills", %{conn: conn} do
      seed()

      {:ok, _view, html} = live(conn, "/skills?project=council-hub")

      assert html =~ "release"
      assert html =~ "issue-writer"
    end
  end
end
