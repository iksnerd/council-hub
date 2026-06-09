package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DigestInput represents the parameters for the project digest tool.
type DigestInput struct {
	Project     string `json:"project"`
	Since       string `json:"since"`
	UnreadOnly  string `json:"unread_only"`
	Agent       string `json:"agent"`
	ClusterWide string `json:"cluster_wide"`
}

// handleGetDigest returns a project activity digest in JSON format.
func (r *Registry) handleGetDigest(ctx context.Context, req *mcp.CallToolRequest, args DigestInput) (*mcp.CallToolResult, ToolOutput, error) {
	if args.ClusterWide == "true" {
		return r.handleGetDigestCluster(args)
	}

	msg := textResult

	// unread_only mode: filter rooms to only those with messages after the agent's stored cursor.
	if args.UnreadOnly == "true" {
		agent := args.Agent
		if agent == "" {
			agent = "default"
		}

		// Look back 30 days to catch all rooms, then filter by cursor.
		allDigest, err := r.Server.GetDigest(args.Project, time.Now().Add(-30*24*time.Hour).UTC().Format("2006-01-02T15:04:05"))
		if err != nil {
			r.Server.Logger.Error("Failed to get digest for unread_only", "error", err)
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}

		// Filter to rooms where latest_message_id is after the stored cursor.
		// UUID v7 IDs sort lexicographically by creation time, so string comparison is valid.
		var filtered []council.DigestEntry
		for _, entry := range allDigest {
			cursor, cursorErr := r.Server.GetCursor(agent, entry.RoomID)
			if cursorErr != nil {
				r.Server.Logger.Error("Failed to get cursor", "agent", agent, "room_id", entry.RoomID, "error", cursorErr)
				continue
			}
			if cursor == "" || (entry.LatestMessageID != "" && entry.LatestMessageID > cursor) {
				entry.LatestExcerpt = digestExcerpt(entry.LatestExcerpt)
				filtered = append(filtered, entry)
			}
		}

		if len(filtered) == 0 {
			return msg(fmt.Sprintf("No unread rooms for agent '%s'. All rooms are up to date.", agent))
		}

		out, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			return msg(fmt.Sprintf("Error formatting JSON: %s", err.Error()))
		}
		return msg(string(out))
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
