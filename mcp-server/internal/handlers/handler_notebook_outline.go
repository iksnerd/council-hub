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
	fmt.Fprintf(&b, "**Notebook:** %s | **Project:** %s | **Entries:** %d\n---\n", notebook.ID, notebook.Project, len(entries))

	if len(entries) == 0 {
		fmt.Fprintf(&b, "\nEmpty notebook. Add entries with edit_notebook(action=add, notebook_id=%s, ref_id=<message_id> or prose=<markdown>).\n", notebook.ID)
		return msg(b.String())
	}

	for _, e := range entries {
		switch {
		case e.Kind == "prose":
			fmt.Fprintf(&b, "\n%s\n*(entry %s)*\n", e.Prose, e.ID)
		case !e.RefFound:
			fmt.Fprintf(&b, "\n⚠ **referenced message #%.8s not found** — deleted, or it lives on another cluster node. *(entry %s)*\n", e.RefID, e.ID)
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

	return msg(b.String())
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
		fmt.Fprintf(b, "- **%s** — %s (%d entries)\n", n.ID, title, n.EntryCount)
	}
}
