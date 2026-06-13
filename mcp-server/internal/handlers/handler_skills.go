package handlers

import (
	"context"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterSkillInput is the write side of the methodology registry: register
// (upsert by name) or, with remove='true', delete a skill.
type RegisterSkillInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	WhenToUse   string `json:"when_to_use"`
	Content     string `json:"content"`
	Project     string `json:"project"`
	Tags        string `json:"tags"`
	Source      string `json:"source"`
	Remove      string `json:"remove"`
}

func (r *Registry) handleRegisterSkill(ctx context.Context, req *mcp.CallToolRequest, args RegisterSkillInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if strings.TrimSpace(args.Name) == "" {
		return msg("Error: name is required.")
	}

	if args.Remove == "true" {
		if err := r.Server.RemoveSkill(args.Name); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		r.Server.Logger.Info("Skill removed", "name", args.Name)
		return msg(fmt.Sprintf("Skill '%s' removed from the registry.", args.Name))
	}

	for _, c := range []struct {
		name, val string
		max       int
	}{
		{"name", args.Name, maxMetadataLen},
		{"description", args.Description, maxMetadataLen},
		{"when_to_use", args.WhenToUse, maxMetadataLen},
		{"tags", args.Tags, maxMetadataLen},
		{"source", args.Source, maxMetadataLen},
		{"content", args.Content, maxContentLen},
	} {
		if err := validateSize(c.name, c.val, c.max); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
	}

	if err := r.Server.RegisterSkill(council.Skill{
		Name:        args.Name,
		Description: args.Description,
		WhenToUse:   args.WhenToUse,
		Content:     args.Content,
		Project:     args.Project,
		Tags:        args.Tags,
		Source:      args.Source,
	}); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}
	r.Server.Logger.Info("Skill registered", "name", args.Name, "project", args.Project)
	return msg(fmt.Sprintf("Skill '%s' registered. Discover it with query_skills_registry(query=…), or query_skills_registry(name=%s) for the full playbook.", args.Name, args.Name))
}

// QuerySkillsInput is the read side: list/filter the registry, or fetch one
// skill's full playbook by name.
type QuerySkillsInput struct {
	Query   string `json:"query"`
	Name    string `json:"name"`
	Project string `json:"project"`
	Tag     string `json:"tag"`
}

func (r *Registry) handleQuerySkillsRegistry(ctx context.Context, req *mcp.CallToolRequest, args QuerySkillsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	// name → render the single skill in full (the playbook body included).
	if strings.TrimSpace(args.Name) != "" {
		sk, err := r.Server.GetSkill(args.Name)
		if err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		return msg(renderSkill(sk))
	}

	skills, err := r.Server.QuerySkills(args.Query, args.Project, args.Tag)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	var b strings.Builder
	b.WriteString("# 🛠 Skills Registry\n")
	var filters []string
	for _, f := range []struct{ name, val string }{{"query", args.Query}, {"project", args.Project}, {"tag", args.Tag}} {
		if strings.TrimSpace(f.val) != "" {
			filters = append(filters, f.name+"="+f.val)
		}
	}
	if len(filters) > 0 {
		fmt.Fprintf(&b, "*Filters: %s*\n", strings.Join(filters, ", "))
	}
	fmt.Fprintf(&b, "**%d skill(s)** — the project task playbook, queryable from inside the DKR.\n", len(skills))

	if len(skills) == 0 {
		b.WriteString("\nNo skills registered yet. Add one with register_skill(name=…, description=…, when_to_use=…, source=…) so the playbook is discoverable from any node.\n")
		return msg(b.String())
	}

	for _, sk := range skills {
		scope := ""
		if sk.Project == "" {
			scope = " [global]"
		}
		fmt.Fprintf(&b, "\n## %s%s\n", sk.Name, scope)
		if sk.Description != "" {
			fmt.Fprintf(&b, "%s\n", sk.Description)
		}
		if sk.WhenToUse != "" {
			fmt.Fprintf(&b, "- **When:** %s\n", sk.WhenToUse)
		}
		if sk.Tags != "" {
			fmt.Fprintf(&b, "- **Tags:** %s\n", sk.Tags)
		}
		if sk.Source != "" {
			fmt.Fprintf(&b, "- **Source:** %s\n", sk.Source)
		}
		if strings.TrimSpace(sk.Content) != "" {
			fmt.Fprintf(&b, "- *(full playbook: query_skills_registry(name=%s))*\n", sk.Name)
		}
	}
	b.WriteString("\n---\n*Registry is node-local. register_skill upserts by name; query_skills_registry(name=…) returns one skill's full playbook.*\n")
	return msg(b.String())
}

// renderSkill formats a single skill's full record, including the playbook body.
func renderSkill(sk council.Skill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 🛠 %s\n", sk.Name)

	scope := sk.Project
	if scope == "" {
		scope = "global"
	}
	fmt.Fprintf(&b, "**Scope:** %s", scope)
	if sk.Tags != "" {
		fmt.Fprintf(&b, " | **Tags:** %s", sk.Tags)
	}
	if sk.Source != "" {
		fmt.Fprintf(&b, " | **Source:** %s", sk.Source)
	}
	b.WriteString("\n")

	if sk.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", sk.Description)
	}
	if sk.WhenToUse != "" {
		fmt.Fprintf(&b, "\n**When to use:** %s\n", sk.WhenToUse)
	}
	if strings.TrimSpace(sk.Content) != "" {
		fmt.Fprintf(&b, "\n---\n%s\n", sk.Content)
	}
	fmt.Fprintf(&b, "\n*Updated %s*\n", sk.UpdatedAt.Format("2006-01-02 15:04"))
	return b.String()
}
