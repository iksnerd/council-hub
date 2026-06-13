package handlers

import (
	"context"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReadNotebookInput represents the parameters for reading a project notebook
// (compiled timeline via project, or a curated outline via notebook_id).
type ReadNotebookInput struct {
	Project     string `json:"project"`
	NotebookID  string `json:"notebook_id"`
	Types       string `json:"types"`
	Since       string `json:"since"`
	Until       string `json:"until"`
	AfterID     string `json:"after_id"`
	Limit       string `json:"limit"`
	Level       string `json:"level"`
	ClusterWide string `json:"cluster_wide"`
}

// parseNotebookTypes splits and validates a CSV of message types. Empty input
// returns nil (the council layer applies the decision/action/synthesis default).
func parseNotebookTypes(csv string) ([]string, error) {
	if strings.TrimSpace(csv) == "" {
		return nil, nil
	}
	var types []string
	for _, t := range strings.Split(csv, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !validMessageTypes[t] {
			return nil, fmt.Errorf("invalid message type '%s'", t)
		}
		types = append(types, t)
	}
	return types, nil
}

func (r *Registry) handleReadNotebook(ctx context.Context, req *mcp.CallToolRequest, args ReadNotebookInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	// Curated outline mode — notebook_id takes precedence over project.
	if args.NotebookID != "" {
		if args.ClusterWide == "true" {
			return msg("Error: notebook outlines are node-local — read them without cluster_wide.")
		}
		level := 0
		if args.Level != "" {
			_, _ = fmt.Sscanf(args.Level, "%d", &level)
		}
		return r.renderOutline(args.NotebookID, level)
	}

	if args.Project == "" {
		return msg("Error: project or notebook_id is required.")
	}

	types, err := parseNotebookTypes(args.Types)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s. Valid types: message, thought, draft, decision, review, action, critique, synthesis, note.", err.Error()))
	}

	if args.ClusterWide == "true" {
		return r.handleReadNotebookCluster(args)
	}

	limit := 0
	if args.Limit != "" {
		_, _ = fmt.Sscanf(args.Limit, "%d", &limit)
	}

	entries, err := r.Server.GetNotebookEntries(args.Project, types, args.Since, args.Until, args.AfterID, limit)
	if err != nil {
		r.Server.Logger.Error("Failed to get notebook entries", "project", args.Project, "error", err)
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	if len(entries) == 0 {
		roomCount, countErr := r.Server.CountRoomsInProject(args.Project)
		if countErr == nil && roomCount == 0 {
			return msg(fmt.Sprintf("Error: no rooms found for project '%s'.", args.Project))
		}
		return msg(fmt.Sprintf("No notebook entries for project '%s' with types %s. Rooms exist but no messages match — try types=message or a wider time window.", args.Project, describeNotebookTypes(types)))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Notebook — %s\n", args.Project)
	fmt.Fprintf(&b, "**Types:** %s | **Entries:** %d", describeNotebookTypes(types), len(entries))
	if args.Since != "" || args.Until != "" {
		fmt.Fprintf(&b, " | **Window:** %s → %s", orStr(args.Since, "…"), orStr(args.Until, "…"))
	}
	b.WriteString("\n---\n")
	writeNotebookEntries(&b, entries, r.noteConnections(entries))

	// Structured JSON footer for machine-parseable cursor tracking (same shape
	// as read_transcript's after_id mode).
	latest := entries[len(entries)-1].ID
	fmt.Fprintf(&b, "\n```json\n{\"latest_message_id\":\"%s\",\"entry_count\":%d}\n```\n", latest, len(entries))

	r.appendNotebookList(&b, args.Project)

	return msg(b.String())
}

// noteConnections gathers, per note entry, its outgoing connective links
// (informs / relates / refines) so the timeline can weave the journal into the
// deliberation it informs. Only note entries are queried, keeping the work
// bounded to the journal subset of a (capped) timeline.
func (r *Registry) noteConnections(entries []council.NotebookEntry) map[string][]council.MessageLink {
	conns := make(map[string][]council.MessageLink)
	for _, e := range entries {
		if e.MessageType != "note" {
			continue
		}
		if links, err := r.Server.NoteConnections(e.ID); err == nil && len(links) > 0 {
			conns[e.ID] = links
		}
	}
	return conns
}

// writeNotebookEntries renders entries chronologically, grouped under one
// heading per day. Each entry resolves {sha:...} refs against its own room's
// repo and carries a 📌 marker when pinned. conns maps an entry ID to its
// connective links (notes only); pass nil to skip the weave.
func writeNotebookEntries(b *strings.Builder, entries []council.NotebookEntry, conns map[string][]council.MessageLink) {
	day := ""
	for _, e := range entries {
		d := e.Timestamp.Format("2006-01-02")
		if d != day {
			day = d
			fmt.Fprintf(b, "\n## %s\n", day)
		}
		writeNotebookEntry(b, e, "", conns[e.ID])
	}
}

// writeNotebookEntry renders one timeline entry. nodeTag, when non-empty, is
// prefixed in cluster-wide output to show which node the entry came from. conns
// are the entry's connective links (notes wired to deliberation), rendered as
// ↳ lines beneath the entry.
func writeNotebookEntry(b *strings.Builder, e council.NotebookEntry, nodeTag string, conns []council.MessageLink) {
	ts := e.Timestamp.Format("15:04")
	pin := ""
	if e.Pinned {
		pin = " 📌"
	}
	node := ""
	if nodeTag != "" {
		node = fmt.Sprintf("[%s] ", nodeTag)
	}
	content := council.ResolveCommitRefs(e.Content, e.Repo)
	fmt.Fprintf(b, "\n**[#%.8s %s] %s[%s] %s (%s)%s:**\n%s\n", e.ID, ts, node, e.RoomID, e.Author, e.MessageType, pin, content)
	for _, l := range conns {
		fmt.Fprintf(b, "↳ %s #%.8s\n", l.Relation, l.ToID)
	}
}

func describeNotebookTypes(types []string) string {
	if len(types) == 0 {
		return strings.Join(council.DefaultNotebookTypes, ",")
	}
	return strings.Join(types, ",")
}

func orStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
