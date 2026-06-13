package handlers

import (
	"context"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// EditNotebookInput represents the parameters for curating a notebook outline.
type EditNotebookInput struct {
	Action       string `json:"action"`
	NotebookID   string `json:"notebook_id"`
	Project      string `json:"project"`
	Title        string `json:"title"`
	EntryID      string `json:"entry_id"`
	Kind         string `json:"kind"`
	RefID        string `json:"ref_id"`
	Prose        string `json:"prose"`
	AfterEntryID string `json:"after_entry_id"`
}

func (r *Registry) handleEditNotebook(ctx context.Context, req *mcp.CallToolRequest, args EditNotebookInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	switch args.Action {
	case "create":
		if args.NotebookID == "" {
			return msg("Error: notebook_id is required.")
		}
		if err := validateSize("title", args.Title, maxMetadataLen); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		if err := r.Server.CreateNotebook(args.NotebookID, args.Project, args.Title); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		r.Server.Logger.Info("Notebook created", "notebook_id", args.NotebookID, "project", args.Project)
		return msg(fmt.Sprintf("Notebook '%s' created. Add entries with edit_notebook(action=add, notebook_id=%s, ref_id=<message_id>) or prose sections with action=add, prose=<markdown>. Read it with read_notebook(notebook_id=%s).", args.NotebookID, args.NotebookID, args.NotebookID))

	case "delete":
		if args.NotebookID == "" {
			return msg("Error: notebook_id is required.")
		}
		if err := r.Server.DeleteNotebook(args.NotebookID); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		r.Server.Logger.Info("Notebook deleted", "notebook_id", args.NotebookID)
		return msg(fmt.Sprintf("Notebook '%s' deleted (referenced messages are untouched).", args.NotebookID))

	case "add":
		if args.NotebookID == "" {
			return msg("Error: notebook_id is required.")
		}
		if err := validateSize("prose", args.Prose, maxContentLen); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		kind := args.Kind
		if kind == "" {
			// Infer: a ref_id means a ref, prose means a prose section.
			if args.RefID != "" {
				kind = "ref"
			} else {
				kind = "prose"
			}
		}
		entryID, err := r.Server.AddOutlineEntry(args.NotebookID, kind, args.RefID, args.Prose, args.AfterEntryID)
		if err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		return msg(fmt.Sprintf("Entry %s added to notebook '%s' (kind: %s).", entryID, args.NotebookID, kind))

	case "update":
		if args.EntryID == "" {
			return msg("Error: entry_id is required.")
		}
		if err := validateSize("prose", args.Prose, maxContentLen); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		if err := r.Server.UpdateOutlineEntry(args.EntryID, args.Prose); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		return msg(fmt.Sprintf("Entry %s updated.", args.EntryID))

	case "remove":
		if args.EntryID == "" {
			return msg("Error: entry_id is required.")
		}
		if err := r.Server.RemoveOutlineEntry(args.EntryID); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		return msg(fmt.Sprintf("Entry %s removed.", args.EntryID))

	case "move":
		if args.EntryID == "" {
			return msg("Error: entry_id is required.")
		}
		if err := r.Server.MoveOutlineEntry(args.EntryID, args.AfterEntryID); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		if args.AfterEntryID == "" {
			return msg(fmt.Sprintf("Entry %s moved to the top.", args.EntryID))
		}
		return msg(fmt.Sprintf("Entry %s moved after %s.", args.EntryID, args.AfterEntryID))

	default:
		return msg(fmt.Sprintf("Error: unknown action '%s'. Valid actions: create, add, update, remove, move, delete.", args.Action))
	}
}

// renderOutline serves read_notebook(notebook_id=...): the curated outline
// with ref entries transcluded live from the ledger. Entry IDs are shown so
// agents can address them in edit_notebook calls.
func (r *Registry) renderOutline(notebookID string) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	notebook, entries, err := r.Server.GetOutline(notebookID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	var b strings.Builder
	title := notebook.Title
	if title == "" {
		title = notebook.ID
	}
	fmt.Fprintf(&b, "# 📓 %s\n", title)
	scope := notebook.Project
	if scope == "" {
		scope = "global (cross-project)"
	}
	fmt.Fprintf(&b, "**Notebook:** %s | **Project:** %s | **Entries:** %d\n---\n", notebook.ID, scope, len(entries))

	if len(entries) == 0 {
		fmt.Fprintf(&b, "\nEmpty notebook. Add entries with edit_notebook(action=add, notebook_id=%s, ref_id=<message_id> or prose=<markdown>).\n", notebook.ID)
		return msg(b.String())
	}

	// room_ref entries are pulled out and regrouped by their transcluded live
	// status: a notebook of room_refs is a self-sorting work list — In flight
	// vs Done re-partitions because the truth lives in the rooms, never by
	// editing the list. prose and message refs keep their authored positions
	// and render inline (the document spine); the work-list groups follow.
	var inFlight, done []council.OutlineEntry
	for _, e := range entries {
		switch {
		case e.Kind == "room_ref":
			if roomRefDone(e) {
				done = append(done, e)
			} else {
				inFlight = append(inFlight, e)
			}
		case e.Kind == "prose":
			fmt.Fprintf(&b, "\n%s\n*(entry %s)*\n", e.Prose, e.ID)
		case !e.RefFound:
			fmt.Fprintf(&b, "\n⚠ **referenced %s '%.12s' not found** — deleted, or it lives on another cluster node. *(entry %s)*\n", refNoun(e.Kind), e.RefID, e.ID)
		default:
			ts := e.RefTime.Format("2006-01-02 15:04")
			pin := ""
			if e.RefPinned {
				pin = " 📌"
			}
			content := council.ResolveCommitRefs(e.RefContent, e.RefRepo)
			fmt.Fprintf(&b, "\n> **[#%.8s %s] [%s] %s (%s)%s:**\n> %s\n*(entry %s)*\n",
				e.RefID, ts, e.RefRoomID, e.RefAuthor, e.RefType, pin,
				strings.ReplaceAll(content, "\n", "\n> "), e.ID)
		}
	}

	if len(inFlight) > 0 {
		fmt.Fprintf(&b, "\n## 🔄 In flight (%d)\n", len(inFlight))
		for _, e := range inFlight {
			renderRoomRef(&b, e)
		}
	}
	if len(done) > 0 {
		fmt.Fprintf(&b, "\n## ✅ Done (%d)\n", len(done))
		for _, e := range done {
			renderRoomRef(&b, e)
		}
	}

	return msg(b.String())
}

// roomRefDone reports whether a transcluded room_ref counts as finished work.
// A resolved/archived room is Done; everything else (active, paused, or a
// dangling ref that needs attention) is still In flight.
func roomRefDone(e council.OutlineEntry) bool {
	return e.RefFound && (e.RefStatus == "resolved" || e.RefStatus == "archived")
}

// renderRoomRef writes one work-list item: the room's live status, topic, and
// latest decision/action. A dangling room_ref renders as a warning in place.
func renderRoomRef(b *strings.Builder, e council.OutlineEntry) {
	if !e.RefFound {
		fmt.Fprintf(b, "\n⚠ **referenced room '%.12s' not found** — deleted, or it lives on another cluster node. *(entry %s)*\n", e.RefID, e.ID)
		return
	}
	fmt.Fprintf(b, "\n**[%s] %s** — %s\n", e.RefStatus, e.RefID, e.RefTopic)
	if e.RefContent != "" {
		ts := e.RefTime.Format("2006-01-02 15:04")
		excerpt := strings.ReplaceAll(e.RefContent, "\n", " ")
		if len(excerpt) > 240 {
			excerpt = excerpt[:240] + "..."
		}
		fmt.Fprintf(b, "  latest %s [%s %s]: %s\n", e.RefType, ts, e.RefAuthor, council.ResolveCommitRefs(excerpt, e.RefRepo))
	}
	fmt.Fprintf(b, "*(entry %s)*\n", e.ID)
}

func refNoun(kind string) string {
	if kind == "room_ref" {
		return "room"
	}
	return "message"
}

// appendNotebookList writes a footer listing a project's curated notebooks
// under the compiled timeline, so the outline layer is discoverable from the
// timeline view.
func (r *Registry) appendNotebookList(b *strings.Builder, project string) {
	notebooks, err := r.Server.ListNotebooks(project)
	if err != nil || len(notebooks) == 0 {
		return
	}
	b.WriteString("\n---\n**Curated notebooks** (read with read_notebook(notebook_id=...)):\n")
	for _, n := range notebooks {
		title := n.Title
		if title == "" {
			title = "(untitled)"
		}
		scope := ""
		if n.Project == "" {
			scope = " [global]"
		}
		fmt.Fprintf(b, "- **%s**%s — %s (%d entries)\n", n.ID, scope, title, n.EntryCount)
	}
}
