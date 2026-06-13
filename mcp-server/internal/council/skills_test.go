package council

import (
	"strings"
	"testing"
)

func TestRegisterSkillAndGet(t *testing.T) {
	s := setupTestServer(t)

	if err := s.RegisterSkill(Skill{
		Name:        "release",
		Description: "Ship a new version",
		WhenToUse:   "shipping vX.Y.Z",
		Content:     "bump, tag, push",
		Project:     "council-hub",
		Tags:        "release,ci",
		Source:      ".claude/skills/release",
	}); err != nil {
		t.Fatalf("RegisterSkill failed: %v", err)
	}

	got, err := s.GetSkill("release")
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}
	if got.Description != "Ship a new version" || got.WhenToUse != "shipping vX.Y.Z" || got.Content != "bump, tag, push" {
		t.Errorf("skill fields not round-tripped: %+v", got)
	}
	if got.Project != "council-hub" {
		t.Errorf("expected normalized project council-hub, got %q", got.Project)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set: %+v", got)
	}
}

func TestRegisterSkillEmptyNameRejected(t *testing.T) {
	s := setupTestServer(t)
	if err := s.RegisterSkill(Skill{Name: "  "}); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRegisterSkillUpsertPreservesCreatedAt(t *testing.T) {
	s := setupTestServer(t)

	if err := s.RegisterSkill(Skill{Name: "smoke", Description: "first"}); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	first, err := s.GetSkill("smoke")
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}

	if err := s.RegisterSkill(Skill{Name: "smoke", Description: "second"}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	second, err := s.GetSkill("smoke")
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}

	if second.Description != "second" {
		t.Errorf("upsert did not update description: %q", second.Description)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("created_at changed on upsert: %v -> %v", first.CreatedAt, second.CreatedAt)
	}

	// No duplicate row — name is the primary key.
	all, err := s.QuerySkills("", "", "")
	if err != nil {
		t.Fatalf("QuerySkills failed: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 skill after upsert, got %d", len(all))
	}
}

func TestRemoveSkill(t *testing.T) {
	s := setupTestServer(t)
	if err := s.RegisterSkill(Skill{Name: "doomed"}); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if err := s.RemoveSkill("doomed"); err != nil {
		t.Fatalf("RemoveSkill failed: %v", err)
	}
	if _, err := s.GetSkill("doomed"); err == nil {
		t.Error("expected GetSkill to fail after removal")
	}
	if err := s.RemoveSkill("doomed"); err == nil {
		t.Error("expected error removing a non-existent skill")
	}
}

func TestQuerySkillsFilters(t *testing.T) {
	s := setupTestServer(t)
	mustRegister := func(sk Skill) {
		t.Helper()
		if err := s.RegisterSkill(sk); err != nil {
			t.Fatalf("RegisterSkill(%s) failed: %v", sk.Name, err)
		}
	}
	mustRegister(Skill{Name: "release", Project: "council-hub", Tags: "go,release", Description: "ship versions"})
	mustRegister(Skill{Name: "deploy-pi", Project: "nous", Tags: "deploy", Description: "cross-compile for Raspberry Pi"})
	mustRegister(Skill{Name: "issue-writer", Project: "", Tags: "writing", Description: "draft GitHub issues"}) // global

	// project filter returns the project's skills PLUS global ones.
	got, err := s.QuerySkills("", "council-hub", "")
	if err != nil {
		t.Fatalf("QuerySkills failed: %v", err)
	}
	if names := skillNames(got); !equalSet(names, []string{"issue-writer", "release"}) {
		t.Errorf("project filter: expected [issue-writer release], got %v", names)
	}

	// tag is a whole-token match — "go" must not match "golang"-style substrings.
	mustRegister(Skill{Name: "golang-thing", Tags: "golang", Description: "x"})
	got, _ = s.QuerySkills("", "", "go")
	if names := skillNames(got); !equalSet(names, []string{"release"}) {
		t.Errorf("tag=go whole-token: expected [release], got %v", names)
	}

	// query substring across description.
	got, _ = s.QuerySkills("raspberry", "", "")
	if names := skillNames(got); !equalSet(names, []string{"deploy-pi"}) {
		t.Errorf("query=raspberry: expected [deploy-pi], got %v", names)
	}

	// no filters returns everything, name-ordered.
	got, _ = s.QuerySkills("", "", "")
	if len(got) != 4 {
		t.Errorf("expected 4 skills, got %d", len(got))
	}
	if got[0].Name != "deploy-pi" {
		t.Errorf("expected name-ordered results, first was %q", got[0].Name)
	}
}

func skillNames(skills []Skill) []string {
	names := make([]string, len(skills))
	for i, sk := range skills {
		names[i] = sk.Name
	}
	return names
}

func equalSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]bool, len(got))
	for _, g := range got {
		seen[g] = true
	}
	for _, w := range want {
		if !seen[w] {
			return false
		}
	}
	return true
}

// guards against the substring-vs-token tag bug regressing.
func TestQuerySkillsTagBracketing(t *testing.T) {
	s := setupTestServer(t)
	if err := s.RegisterSkill(Skill{Name: "a", Tags: "ci, release"}); err != nil { // spaced tags
		t.Fatalf("register failed: %v", err)
	}
	got, _ := s.QuerySkills("", "", "release")
	if !strings.Contains(strings.Join(skillNames(got), ","), "a") {
		t.Errorf("spaced tag not matched: %v", skillNames(got))
	}
}
