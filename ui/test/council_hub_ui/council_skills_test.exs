defmodule CouncilHubUi.CouncilSkillsTest do
  use CouncilHubUi.DataCase

  alias CouncilHubUi.Council
  import CouncilHubUi.CouncilFixtures

  defp seed do
    create_skill(%{
      name: "release",
      project: "council-hub",
      tags: "go,release",
      description: "ship versions"
    })

    create_skill(%{
      name: "deploy-pi",
      project: "nous",
      tags: "deploy",
      description: "cross-compile for Raspberry Pi"
    })

    create_skill(%{
      name: "issue-writer",
      project: "",
      tags: "writing",
      description: "draft GitHub issues"
    })

    create_skill(%{name: "golang-thing", project: "", tags: "golang", description: "x"})
  end

  defp names(skills), do: skills |> Enum.map(& &1.name) |> Enum.sort()

  describe "list_skills/1" do
    test "no filters returns everything, name-ordered" do
      seed()
      skills = Council.list_skills(%{})
      assert names(skills) == ["deploy-pi", "golang-thing", "issue-writer", "release"]
      assert Enum.map(skills, & &1.name) == Enum.sort(Enum.map(skills, & &1.name))
    end

    test "project filter returns the project's skills plus global ones" do
      seed()
      skills = Council.list_skills(%{project: "council-hub"})
      assert names(skills) == ["golang-thing", "issue-writer", "release"]
    end

    test "tag is a whole-token match, not a substring" do
      seed()
      assert names(Council.list_skills(%{tag: "go"})) == ["release"]
      assert names(Council.list_skills(%{tag: "golang"})) == ["golang-thing"]
    end

    test "tag match tolerates spaced tag lists" do
      create_skill(%{name: "spaced", tags: "ci, release"})
      assert names(Council.list_skills(%{tag: "release"})) == ["spaced"]
    end

    test "query is a case-insensitive substring across fields" do
      seed()
      assert names(Council.list_skills(%{query: "RASPBERRY"})) == ["deploy-pi"]
    end
  end

  describe "get_skill/1 and list_skill_projects/0" do
    test "get_skill returns the row or nil" do
      seed()
      assert Council.get_skill("release").description == "ship versions"
      assert Council.get_skill("nope") == nil
    end

    test "list_skill_projects returns distinct non-empty projects" do
      seed()
      assert Council.list_skill_projects() == ["council-hub", "nous"]
    end
  end
end
