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
	Project      string `json:"project"`
	Since        string `json:"since"`
	UnreadOnly   string `json:"unread_only"`
	Agent        string `json:"agent"`
	ClusterWide  string `json:"cluster_wide"`
	ExcludeStale string `json:"exclude_stale"`
}

// digestSummary is the at-a-glance health header prepended to the digest so an
// agent can triage at scale ("35 stale, skip the graveyard") without scanning
// every room. Counts are over the full result set, before exclude_stale hides rows.
type digestSummary struct {
	Total          int `json:"total"`
	WithUnread     int `json:"with_unread"`
	Stale          int `json:"stale"`
	NeedsSynthesis int `json:"needs_synthesis"`
	StalePin       int `json:"stale_pin"`
	Incoherent     int `json:"incoherent"`
	HiddenStale    int `json:"hidden_stale,omitempty"`
}

// digestResponse wraps the rooms array with a summary header. Hint is set only
// when a project filter matched nothing — the `project` filter is an exact field
// match, so a room created without its project set returns a silent zero, and the
// session-start ritual ("call get_digest first") gives no clue why.
type digestResponse struct {
	Summary digestSummary         `json:"summary"`
	Rooms   []council.DigestEntry `json:"rooms"`
	Hint    string                `json:"hint,omitempty"`
}

// digestNoMatchHint points an agent at keyword search when an exact-match project
// filter returns nothing — the common cause is a room whose `project` field was
// never set (see get_or_create_room metadata backfill).
func digestNoMatchHint(project string, matched int) string {
	if project == "" || matched > 0 {
		return ""
	}
	return fmt.Sprintf("No rooms with project='%s'. The project filter is an exact field match, not a keyword search — a room created without its project set won't appear here. Try list_rooms(search='%s') to find it by ID/topic/tag, then get_or_create_room(id=…, project='%s') to backfill the project field.", project, project, project)
}

// summarizeDigest tallies health flags across digest entries.
func summarizeDigest(entries []council.DigestEntry) digestSummary {
	s := digestSummary{Total: len(entries)}
	for _, e := range entries {
		if e.NewMessages > 0 {
			s.WithUnread++
		}
		if digestHasTag(e.Tags, "stale") {
			s.Stale++
		}
		if digestHasTag(e.Tags, "needs-synthesis") {
			s.NeedsSynthesis++
		}
		if digestHasTag(e.Tags, "stale-pin") {
			s.StalePin++
		}
		if digestHasTag(e.Tags, "incoherent") {
			s.Incoherent++
		}
	}
	return s
}

// digestHasTag reports exact (not substring) membership in a comma-separated tag
// list, so "stale" does not match "stale-pin"/"stale-plan".
func digestHasTag(tags, tag string) bool {
	for _, t := range strings.Split(tags, ",") {
		if strings.TrimSpace(t) == tag {
			return true
		}
	}
	return false
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

		out, err := json.MarshalIndent(digestResponse{Summary: summarizeDigest(filtered), Rooms: filtered, Hint: digestNoMatchHint(args.Project, len(filtered))}, "", "  ")
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

	// Summary is computed over the full set so the stale count is honest even when
	// exclude_stale hides those rooms from the list. exclude_stale drops rooms
	// flagged `stale` (the inactive-room graveyard) — but keeps a `stale` room that
	// has genuinely new activity, and never hides stale-pin/stale-plan (those are
	// drift flags on live rooms, not the graveyard).
	summary := summarizeDigest(digest)
	rooms := digest
	if args.ExcludeStale == "true" {
		kept := rooms[:0:0]
		for _, e := range digest {
			if digestHasTag(e.Tags, "stale") && e.NewMessages == 0 {
				continue
			}
			kept = append(kept, e)
		}
		summary.HiddenStale = len(digest) - len(kept)
		rooms = kept
	}

	// Hint reflects whether the project filter matched any room at all (len(digest)),
	// not how many survived exclude_stale (len(rooms)) — a project with only stale
	// rooms still matched, so it isn't the "wrong project name" footgun.
	out, err := json.MarshalIndent(digestResponse{Summary: summary, Rooms: rooms, Hint: digestNoMatchHint(args.Project, len(digest))}, "", "  ")
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
				return council.TruncateRunes(heading, 120, "", 0)
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
	return council.TruncateRunes(flat, 120, " ", 80)
}
