package handlers

import (
	"context"
	"strings"
	"testing"
)

func registerSkill(t *testing.T, reg *Registry, args RegisterSkillInput) string {
	t.Helper()
	res, _, err := reg.handleRegisterSkill(context.Background(), nil, args)
	if err != nil {
		t.Fatalf("handleRegisterSkill(%+v) failed: %v", args, err)
	}
	return resultText(res)
}

func querySkills(t *testing.T, reg *Registry, args QuerySkillsInput) string {
	t.Helper()
	res, _, err := reg.handleQuerySkillsRegistry(context.Background(), nil, args)
	if err != nil {
		t.Fatalf("handleQuerySkillsRegistry(%+v) failed: %v", args, err)
	}
	return resultText(res)
}

func TestHandleRegisterSkillRequiresName(t *testing.T) {
	reg := setupHandlerTest(t)
	if text := registerSkill(t, reg, RegisterSkillInput{Description: "no name"}); !strings.Contains(text, "name is required") {
		t.Errorf("expected name-required error, got: %s", text)
	}
}

func TestHandleRegisterAndQueryByName(t *testing.T) {
	reg := setupHandlerTest(t)
	registerSkill(t, reg, RegisterSkillInput{
		Name:        "release",
		Description: "Ship a new version",
		WhenToUse:   "shipping vX.Y.Z",
		Content:     "## Steps\n1. bump\n2. tag",
		Project:     "council-hub",
		Tags:        "release,ci",
		Source:      ".claude/skills/release",
	})

	// Full-playbook view by name includes the content body.
	text := querySkills(t, reg, QuerySkillsInput{Name: "release"})
	for _, want := range []string{"# 🛠 release", "Ship a new version", "When to use:", "## Steps", ".claude/skills/release"} {
		if !strings.Contains(text, want) {
			t.Errorf("query by name missing %q, got: %s", want, text)
		}
	}
}

func TestHandleQuerySkillsListAndFilters(t *testing.T) {
	reg := setupHandlerTest(t)
	registerSkill(t, reg, RegisterSkillInput{Name: "release", Project: "council-hub", Tags: "release", Description: "ship"})
	registerSkill(t, reg, RegisterSkillInput{Name: "issue-writer", Description: "draft issues"}) // global

	// Catalog view lists names; global is tagged.
	text := querySkills(t, reg, QuerySkillsInput{})
	if !strings.Contains(text, "## release") || !strings.Contains(text, "## issue-writer") {
		t.Errorf("catalog missing skills, got: %s", text)
	}
	if !strings.Contains(text, "issue-writer [global]") {
		t.Errorf("expected global marker on issue-writer, got: %s", text)
	}
	if !strings.Contains(text, "2 skill(s)") {
		t.Errorf("expected count of 2, got: %s", text)
	}

	// Filter echoed back; project view includes global.
	text = querySkills(t, reg, QuerySkillsInput{Project: "council-hub"})
	if !strings.Contains(text, "Filters: project=council-hub") {
		t.Errorf("filter not echoed, got: %s", text)
	}
	if !strings.Contains(text, "## release") || !strings.Contains(text, "## issue-writer") {
		t.Errorf("project view should include the project's skills plus global, got: %s", text)
	}
}

func TestHandleQuerySkillsEmptyRegistry(t *testing.T) {
	reg := setupHandlerTest(t)
	text := querySkills(t, reg, QuerySkillsInput{})
	if !strings.Contains(text, "No skills registered yet") {
		t.Errorf("expected empty-registry hint, got: %s", text)
	}
}

func TestHandleQuerySkillsUnknownName(t *testing.T) {
	reg := setupHandlerTest(t)
	text := querySkills(t, reg, QuerySkillsInput{Name: "nope"})
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not-found error, got: %s", text)
	}
}

func TestHandleRegisterSkillRemove(t *testing.T) {
	reg := setupHandlerTest(t)
	registerSkill(t, reg, RegisterSkillInput{Name: "doomed", Description: "x"})

	text := registerSkill(t, reg, RegisterSkillInput{Name: "doomed", Remove: "true"})
	if !strings.Contains(text, "removed from the registry") {
		t.Errorf("expected removal confirmation, got: %s", text)
	}
	if text := querySkills(t, reg, QuerySkillsInput{Name: "doomed"}); !strings.Contains(text, "not found") {
		t.Errorf("skill should be gone after removal, got: %s", text)
	}
}
