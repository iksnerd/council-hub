package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DigestInput represents the parameters for the project digest tool.
type DigestInput struct {
	Project     string `json:"project"`
	Since       string `json:"since"`
	ClusterWide string `json:"cluster_wide"`
}

// handleGetDigest returns a project activity digest.
func (r *Registry) handleGetDigest(ctx context.Context, req *mcp.CallToolRequest, args DigestInput) (*mcp.CallToolResult, ToolOutput, error) {
	if args.ClusterWide == "true" {
		return r.handleGetDigestCluster(args)
	}

	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.Since == "" {
		return msg("Error: since is required (ISO timestamp, e.g. 2026-03-31T12:00:00).")
	}

	digest, err := r.Server.GetDigest(args.Project, args.Since)
	if err != nil {
		r.Server.Logger.Error("Failed to get digest", "project", args.Project, "since", args.Since, "error", err)
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	if len(digest) == 0 {
		projectNote := ""
		if args.Project != "" {
			projectNote = fmt.Sprintf(" in project '%s'", args.Project)
		}
		return msg(fmt.Sprintf("No new activity or health warnings%s since %s.", projectNote, args.Since))
	}

	var b strings.Builder
	projectNote := ""
	if args.Project != "" {
		projectNote = fmt.Sprintf(" [%s]", args.Project)
	}
	fmt.Fprintf(&b, "# Activity & Knowledge Digest%s \u2014 since %s\n\n", projectNote, args.Since)

	activeCount := 0
	for _, d := range digest {
		if d.NewMessages > 0 {
			activeCount++
		}
	}

	fmt.Fprintf(&b, "### 📈 New Activity (%d rooms)\n", activeCount)
	for _, d := range digest {
		if d.NewMessages > 0 {
			excerpt := digestExcerpt(d.LatestExcerpt)
			healthStr := ""
			if d.SynthesisCount > 0 {
				healthStr = " \U0001f4da[Compiled]"
			}
			fmt.Fprintf(&b, "- **%s** | %d new msg(s)%s | %s: %s\n", d.RoomID, d.NewMessages, healthStr, d.LatestAuthor, excerpt)
		}
	}

	needsAttentionCount := 0
	for _, d := range digest {
		if strings.Contains(d.Tags, "stale") || strings.Contains(d.Tags, "needs-synthesis") {
			needsAttentionCount++
		}
	}

	if needsAttentionCount > 0 {
		fmt.Fprintf(&b, "\n### 🏥 Knowledge Health (%d rooms need attention)\n", needsAttentionCount)
		for _, d := range digest {
			isStale := strings.Contains(d.Tags, "stale")
			needsSyn := strings.Contains(d.Tags, "needs-synthesis")
			if isStale || needsSyn {
				reasons := []string{}
				if isStale {
					reasons = append(reasons, "Stale (>7 days)")
				}
				if needsSyn {
					reasons = append(reasons, fmt.Sprintf("Needs Synthesis (%d decisions)", d.DecisionCount))
				}
				fmt.Fprintf(&b, "- **%s** | ⚠️  %s\n", d.RoomID, strings.Join(reasons, ", "))
			}
		}
	}

	return msg(b.String())
}

// digestExcerpt extracts a clean one-line summary from message content.
// Prefers the first markdown heading, then the first non-empty sentence,
// then falls back to a word-boundary truncation at 120 chars.
func digestExcerpt(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	// Try first markdown heading (## Heading or # Heading)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			heading := strings.TrimLeft(line, "# ")
			if heading != "" {
				if len(heading) > 120 {
					heading = heading[:120] + "..."
				}
				return heading
			}
		}
		// Stop looking after first non-empty non-heading line
		if line != "" {
			break
		}
	}

	// Try first sentence (ends with . ! ?)
	flat := strings.ReplaceAll(content, "\n", " ")
	for i, ch := range flat {
		if (ch == '.' || ch == '!' || ch == '?') && i > 10 {
			sentence := strings.TrimSpace(flat[:i+1])
			if len(sentence) <= 150 {
				return sentence
			}
			break
		}
	}

	// Fallback: word-boundary truncation
	if len(flat) > 120 {
		truncated := flat[:120]
		if i := strings.LastIndex(truncated, " "); i > 80 {
			truncated = truncated[:i]
		}
		return truncated + "..."
	}
	return flat
}
