package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DigestInput represents the parameters for the project digest tool.
type DigestInput struct {
	Project     string `json:"project"`
	Since       string `json:"since"`
	ClusterWide string `json:"cluster_wide"`
}

// handleGetDigest returns a project activity digest in JSON format.
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
		args.Since = time.Now().Add(-24 * time.Hour).UTC().Format("2006-01-02T15:04:05")
	}

	digest, err := r.Server.GetDigest(args.Project, args.Since)
	if err != nil {
		r.Server.Logger.Error("Failed to get digest", "project", args.Project, "since", args.Since, "error", err)
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	for i := range digest {
		digest[i].LatestExcerpt = digestExcerpt(digest[i].LatestExcerpt)
	}

	out, err := json.MarshalIndent(digest, "", "  ")
	if err != nil {
		return msg(fmt.Sprintf("Error formatting JSON: %s", err.Error()))
	}

	return msg(string(out))
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
