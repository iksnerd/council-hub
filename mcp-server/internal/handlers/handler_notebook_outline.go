package handlers

import (
	"context"
	"errors"
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
		// Only kind="ref" points at a message ID (room_ref/query_ref point at a
		// room, task has no ref_id) — resolve a possibly-truncated prefix.
		if kind == "ref" {
			resolved, rerr := r.resolveSingleID(args.RefID)
			if rerr != nil {
				return msg(fmt.Sprintf("Error: %s", rerr.Error()))
			}
			args.RefID = resolved
		}
		entryID, err := r.Server.AddOutlineEntry(args.NotebookID, kind, args.RefID, args.Prose, args.AfterEntryID)
		if err != nil {
			var dup *council.ErrAlreadyReferenced
			if errors.As(err, &dup) {
				return msg(fmt.Sprintf("Already referenced (no-op) — %s '%s' is entry %s in notebook '%s'. Not re-added.", dup.Kind, dup.RefID, dup.EntryID, args.NotebookID))
			}
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

	case "start", "check", "uncheck":
		if args.EntryID == "" {
			return msg("Error: entry_id is required.")
		}
		status := map[string]string{"start": "doing", "check": "done", "uncheck": "open"}[args.Action]
		if err := r.Server.SetTaskStatus(args.EntryID, status); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		label := map[string]string{"doing": "in progress 🔄", "done": "done ☑", "open": "open ☐"}[status]
		return msg(fmt.Sprintf("Task %s marked %s.", args.EntryID, label))

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
		return msg(fmt.Sprintf("Error: unknown action '%s'. Valid actions: create, add, update, check, uncheck, remove, move, delete.", args.Action))
	}
}

// renderOutline serves read_notebook(notebook_id=...): the curated outline
// with ref entries transcluded live from the ledger. Entry IDs are shown so
// agents can address them in edit_notebook calls.
//
// level is an NLS-style structural ViewSpec (the counterpart to the transcript
// line-one truncate): level 0 renders everything; level N collapses each prose
// entry to its heading skeleton down to depth N (a table of contents) and clips
// transcluded message bodies to their first line. The self-sorting task and
// room_ref groups are structural leaves and always render.
func (r *Registry) renderOutline(notebookID string, level int) (*mcp.CallToolResult, ToolOutput, error) {
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

	// Self-sorting entries are pulled out and regrouped so the notebook stays true
	// without hand-editing: tasks by their own done state (☐ Open / ☑ Done), and
	// room_refs by their transcluded live status (🔄 In flight / ✅ Done — the truth
	// lives in the rooms). prose and message refs keep their authored positions and
	// render inline (the document spine); the work-list groups follow.
	var doingTasks, openTasks, doneTasks, inFlight, doneRooms []council.OutlineEntry
	for _, e := range entries {
		switch {
		case e.Kind == "task":
			switch e.Status {
			case "done":
				doneTasks = append(doneTasks, e)
			case "doing":
				doingTasks = append(doingTasks, e)
			default:
				openTasks = append(openTasks, e)
			}
		case e.Kind == "room_ref":
			if roomRefDone(e) {
				doneRooms = append(doneRooms, e)
			} else {
				inFlight = append(inFlight, e)
			}
		case e.Kind == "query_ref":
			// A live query transclusion: "latest <type> in <room>", resolved now —
			// structural addressing, not a frozen message ID. Renders inline.
			roomID, msgType := e.RefID, ""
			if i := strings.LastIndex(e.RefID, ":"); i > 0 {
				roomID, msgType = e.RefID[:i], e.RefID[i+1:]
			}
			if !e.RefFound {
				fmt.Fprintf(&b, "\n> **↪ latest %s in %s** — *none yet* *(entry %s)*\n", msgType, roomID, e.ID)
			} else {
				ts := e.RefTime.Format("2006-01-02 15:04")
				content := council.ResolveCommitRefs(e.RefContent, e.RefRepo)
				if level > 0 {
					content = firstNonEmptyLine(content)
				}
				fmt.Fprintf(&b, "\n> **↪ latest %s in %s** — [%s] %s:\n> %s\n*(entry %s)*\n",
					msgType, roomID, ts, e.RefAuthor, strings.ReplaceAll(content, "\n", "\n> "), e.ID)
			}
		case e.Kind == "prose":
			prose := e.Prose
			if level > 0 {
				prose = clipProseToLevel(prose, level)
			}
			fmt.Fprintf(&b, "\n%s\n*(entry %s)*\n", prose, e.ID)
		case !e.RefFound:
			fmt.Fprintf(&b, "\n⚠ **referenced %s '%.12s' not found** — deleted, or it lives on another cluster node. *(entry %s)*\n", refNoun(e.Kind), e.RefID, e.ID)
		default:
			ts := e.RefTime.Format("2006-01-02 15:04")
			pin := ""
			if e.RefPinned {
				pin = " 📌"
			}
			content := council.ResolveCommitRefs(e.RefContent, e.RefRepo)
			if level > 0 {
				content = firstNonEmptyLine(content)
			}
			fmt.Fprintf(&b, "\n> **[#%.8s %s] [%s] %s (%s)%s:**\n> %s\n*(entry %s)*\n",
				e.RefID, ts, e.RefRoomID, e.RefAuthor, e.RefType, pin,
				strings.ReplaceAll(content, "\n", "\n> "), e.ID)
		}
	}

	if len(doingTasks) > 0 {
		fmt.Fprintf(&b, "\n## 🔄 In progress (%d)\n", len(doingTasks))
		for _, e := range doingTasks {
			renderTask(&b, e)
		}
	}
	if len(openTasks) > 0 {
		fmt.Fprintf(&b, "\n## ☐ Open (%d)\n", len(openTasks))
		for _, e := range openTasks {
			renderTask(&b, e)
		}
	}
	if len(doneTasks) > 0 {
		fmt.Fprintf(&b, "\n## ☑ Done (%d)\n", len(doneTasks))
		for _, e := range doneTasks {
			renderTask(&b, e)
		}
	}
	if len(inFlight) > 0 {
		fmt.Fprintf(&b, "\n## 🔄 In flight (%d)\n", len(inFlight))
		for _, e := range inFlight {
			renderRoomRef(&b, e)
		}
	}
	if len(doneRooms) > 0 {
		fmt.Fprintf(&b, "\n## ✅ Done (%d)\n", len(doneRooms))
		for _, e := range doneRooms {
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
		excerpt = council.TruncateRunes(excerpt, 240, "", 0)
		fmt.Fprintf(b, "  latest %s [%s %s]: %s\n", e.RefType, ts, e.RefAuthor, council.ResolveCommitRefs(excerpt, e.RefRepo))
	}
	fmt.Fprintf(b, "*(entry %s)*\n", e.ID)
}

// renderTask writes one checklist item — a markdown checkbox carrying the task
// label and its addressable entry ID (so it can be started, checked, or edited).
// The box reflects status: [ ] open, [~] doing, [x] done.
func renderTask(b *strings.Builder, e council.OutlineEntry) {
	box := "[ ]"
	switch e.Status {
	case "done":
		box = "[x]"
	case "doing":
		box = "[~]"
	}
	label := strings.ReplaceAll(strings.TrimSpace(e.Prose), "\n", " ")
	fmt.Fprintf(b, "- %s %s *(entry %s)*\n", box, label, e.ID)
}

func refNoun(kind string) string {
	if kind == "room_ref" {
		return "room"
	}
	return "message"
}

// clipProseToLevel projects a prose entry to its heading skeleton: only
// markdown headings (#, ##, …) of depth <= level survive; body text and deeper
// headings collapse, marked with an ellipsis. A prose block with no heading
// within the level shows its first line as a stub. The structural ViewSpec
// counterpart to the transcript line-one truncate — a table of contents over a
// curated notebook.
func clipProseToLevel(prose string, level int) string {
	var kept []string
	clipped := false
	for _, line := range strings.Split(prose, "\n") {
		trimmed := strings.TrimSpace(line)
		if d := headingDepth(trimmed); d > 0 && d <= level {
			kept = append(kept, trimmed)
			continue
		}
		if trimmed != "" {
			clipped = true
		}
	}
	if len(kept) == 0 {
		if first := firstNonEmptyLine(prose); first != "" {
			return first + " …"
		}
		return ""
	}
	out := strings.Join(kept, "\n")
	if clipped {
		out += "\n…"
	}
	return out
}

// headingDepth returns the markdown heading depth of a line (1 for "# ", 2 for
// "## ", …), or 0 if the line is not a heading. A hash run must be followed by
// a space to count, so "#tag" is not a heading.
func headingDepth(line string) int {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n > 0 && n < len(line) && line[n] == ' ' {
		return n
	}
	return 0
}

// firstNonEmptyLine returns the first non-blank line of s, trimmed.
func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
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
