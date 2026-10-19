package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// EditNotebookInput represents the parameters for curating a notebook outline.
type EditNotebookInput struct {
	Action     string `json:"action"`
	NotebookID string `json:"notebook_id"`
	Project    string `json:"project"`
	Title      string `json:"title"`
	EntryID    string `json:"entry_id"`
	Kind       string `json:"kind"`
	RefID      string `json:"ref_id"`
	// RoomID is an accepted alias for RefID. For kind=room_ref the value *is* a
	// room id, and room_id is what every room-scoped tool calls it — so it is
	// the spelling an agent carries in from the call it just made (#01a0b156).
	RoomID       string `json:"room_id"`
	RefIDs       string `json:"ref_ids"`
	Prose        string `json:"prose"`
	AfterEntryID string `json:"after_entry_id"`
	// Confirm guards whole-notebook delete and purge: it must equal the
	// notebook id (or its current entry count).
	Confirm string `json:"confirm"`
}

// completionMarkerRe matches a task label that opens with a "this is finished"
// marker (DONE, done:, COMPLETE, ✅, [x]). Adding such a task does not close
// anything: a new task starts open, and only action=check closes one.
var completionMarkerRe = regexp.MustCompile(`(?i)^\s*(\[x\]|✅|done\b|complete(d)?\b)`)

// foreignArgs rejects arguments that belong to a different action. A silently
// ignored argument is how delete+entry_id wiped a 419-entry notebook (#01a14ee7):
// the caller meant "remove this entry" and got "delete the notebook".
func foreignArgs(args EditNotebookInput) string {
	type field struct{ name, val string }
	whole := []field{{"entry_id", args.EntryID}, {"ref_id", args.RefID}, {"room_id", args.RoomID},
		{"ref_ids", args.RefIDs}, {"prose", args.Prose}, {"kind", args.Kind}, {"after_entry_id", args.AfterEntryID}}
	switch args.Action {
	case "delete", "restore", "purge":
		for _, f := range whole {
			if f.val == "" {
				continue
			}
			if f.name == "entry_id" && args.Action == "delete" {
				return "Error: delete removes the whole notebook; did you mean remove? (action=remove, entry_id=... drops one entry.)"
			}
			return fmt.Sprintf("Error: action=%s takes notebook_id (and confirm), not %s — it acts on the whole notebook.", args.Action, f.name)
		}
	case "create":
		for _, f := range whole {
			if f.val != "" {
				return fmt.Sprintf("Error: action=create takes notebook_id, project and title, not %s.", f.name)
			}
		}
	}
	if args.Confirm != "" && args.Action != "delete" && args.Action != "purge" {
		return fmt.Sprintf("Error: confirm only applies to action=delete or purge, not %s.", args.Action)
	}
	return ""
}

// entryCount returns a notebook's current entry count, or -1 if unreadable.
func (r *Registry) entryCount(notebookID string) int {
	if notebookID == "" {
		return -1
	}
	n, err := r.Server.GetNotebook(notebookID)
	if err != nil {
		return -1
	}
	return n.EntryCount
}

// notebookOf resolves the notebook a write targets: the given notebook_id, or
// the owner of entry_id.
func (r *Registry) notebookOf(notebookID, entryID string) string {
	if notebookID != "" {
		return notebookID
	}
	if entryID == "" {
		return ""
	}
	nb, _ := r.Server.EntryNotebookID(entryID)
	return nb
}

// countsNote renders "Entries: before → after." for a write's response.
func countsNote(before, after int) string {
	if before < 0 || after < 0 {
		return ""
	}
	return fmt.Sprintf(" Entries: %d → %d.", before, after)
}

var snapshotNameRe = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// snapshotNotebook writes the notebook as markdown into the archives dir before
// it is deleted, so even a mistaken delete leaves a readable copy on disk.
// Returns the path, or "" with a reason when there is nowhere to write.
func (r *Registry) snapshotNotebook(notebookID string) (string, string) {
	dir := r.Server.NotebookSnapshotDir()
	if dir == "" {
		return "", "no snapshot: in-memory database has no archives directory"
	}
	text, err := r.outlineText(notebookID, 0, council.OutlineFilter{})
	if err != nil {
		return "", "snapshot failed: " + err.Error()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "snapshot failed: " + err.Error()
	}
	name := fmt.Sprintf("%s-%s.md", snapshotNameRe.ReplaceAllString(notebookID, "_"), time.Now().UTC().Format("20060102T150405Z"))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", "snapshot failed: " + err.Error()
	}
	return path, ""
}

func (r *Registry) handleEditNotebook(ctx context.Context, req *mcp.CallToolRequest, args EditNotebookInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if e := foreignArgs(args); e != "" {
		return msg(e)
	}

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
		return msg(fmt.Sprintf("Notebook '%s' created (0 entries). Add entries with edit_notebook(action=add, notebook_id=%s, ref_id=<message_id>) or prose sections with action=add, prose=<markdown>. Read it with read_notebook(notebook_id=%s).", args.NotebookID, args.NotebookID, args.NotebookID))

	case "delete", "purge":
		if args.NotebookID == "" {
			return msg("Error: notebook_id is required.")
		}
		count := -1
		if args.Action == "delete" {
			count = r.entryCount(args.NotebookID)
			if count < 0 {
				_, err := r.Server.GetNotebook(args.NotebookID)
				return msg(fmt.Sprintf("Error: %s", err.Error()))
			}
		} else if n, err := r.Server.GetNotebookIncludingDeleted(args.NotebookID); err == nil {
			count = n.EntryCount
		}
		if count >= 0 && args.Confirm != args.NotebookID && args.Confirm != strconv.Itoa(count) {
			return msg(fmt.Sprintf("Error: action=%s affects the WHOLE notebook '%s' (%d entries). Pass confirm=%s (the notebook id) or confirm=%d (its entry count) to proceed. To drop a single entry use action=remove, entry_id=...", args.Action, args.NotebookID, count, args.NotebookID, count))
		}
		if args.Action == "purge" {
			removed, err := r.Server.PurgeNotebook(args.NotebookID)
			if err != nil {
				return msg(fmt.Sprintf("Error: %s", err.Error()))
			}
			r.Server.Logger.Warn("Notebook purged", "notebook_id", args.NotebookID, "entries", removed)
			return msg(fmt.Sprintf("Notebook '%s' PURGED: %d entries permanently removed (referenced messages are untouched). This cannot be undone.", args.NotebookID, removed))
		}
		snapPath, snapNote := r.snapshotNotebook(args.NotebookID)
		removed, err := r.Server.DeleteNotebook(args.NotebookID)
		if err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		r.Server.Logger.Info("Notebook soft-deleted", "notebook_id", args.NotebookID, "entries", removed)
		out := fmt.Sprintf("Notebook '%s' deleted: %d entries removed from view (referenced messages are untouched). Soft delete — entries are kept; undo with edit_notebook(action=restore, notebook_id=%s), or hard-delete with action=purge.", args.NotebookID, removed, args.NotebookID)
		if snapPath != "" {
			out += " Markdown snapshot: " + snapPath
		} else if snapNote != "" {
			out += " (" + snapNote + ")"
		}
		return msg(out)

	case "restore":
		if args.NotebookID == "" {
			return msg("Error: notebook_id is required.")
		}
		count, err := r.Server.RestoreNotebook(args.NotebookID)
		if err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		return msg(fmt.Sprintf("Notebook '%s' restored with %d entries.", args.NotebookID, count))

	case "add":
		if args.NotebookID == "" {
			return msg("Error: notebook_id is required.")
		}
		if err := validateSize("prose", args.Prose, maxContentLen); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		refID, aerr := resolveAlias(args.RefID, "ref_id", args.RoomID, "room_id")
		if aerr != nil {
			return msg("Error: " + aerr.Error())
		}
		args.RefID = refID

		if args.RefID != "" && args.RefIDs != "" {
			return msg("Error: pass ref_id or ref_ids, not both.")
		}
		// Say which parameter is missing rather than looking up "" and blaming
		// the room: "room '' not found" reads as "that room does not exist",
		// which is the one thing that was never true.
		if (args.Kind == "room_ref" || args.Kind == "ref" || args.Kind == "query_ref") &&
			args.RefID == "" && args.RefIDs == "" {
			return msg(fmt.Sprintf("Error: kind=%s needs the target in ref_id (alias: room_id), or several in ref_ids. None was supplied.", args.Kind))
		}
		kind := args.Kind
		if kind == "" {
			// Infer: a ref means a ref, prose means a prose section.
			if args.RefID != "" || args.RefIDs != "" {
				kind = "ref"
			} else {
				kind = "prose"
			}
		}

		// Batch add: one call per target was four round trips to put four
		// room_refs in a notebook. Mirrors update_room's room_ids convention.
		before := r.entryCount(args.NotebookID)
		if args.RefIDs != "" {
			if args.Prose != "" {
				return msg("Error: ref_ids adds transcluding entries; prose belongs to a single add.")
			}
			refs := splitIDList(args.RefIDs)
			if len(refs) == 0 {
				return msg("Error: ref_ids contained no usable IDs.")
			}

			var added, skipped, failed []string
			// Each insert anchors after the previous one, so the batch lands in
			// the order given — anchoring them all to the same entry would
			// reverse it.
			anchor := args.AfterEntryID
			for _, ref := range refs {
				if kind == "ref" {
					if err := r.resolveInto(&ref); err != nil {
						failed = append(failed, fmt.Sprintf("%s (%s)", ref, err.Error()))
						continue
					}
				}
				entryID, err := r.Server.AddOutlineEntry(args.NotebookID, kind, ref, "", anchor)
				if err != nil {
					var dup *council.ErrAlreadyReferenced
					if errors.As(err, &dup) {
						skipped = append(skipped, fmt.Sprintf("%s (already entry %s)", ref, dup.EntryID))
						continue
					}
					failed = append(failed, fmt.Sprintf("%s (%s)", ref, err.Error()))
					continue
				}
				added = append(added, entryID)
				anchor = entryID
			}

			var b strings.Builder
			fmt.Fprintf(&b, "Batch add to notebook '%s' (kind: %s): %d added", args.NotebookID, kind, len(added))
			if len(skipped) > 0 {
				fmt.Fprintf(&b, ", %d already referenced", len(skipped))
			}
			if len(failed) > 0 {
				fmt.Fprintf(&b, ", %d failed", len(failed))
			}
			b.WriteString(".")
			b.WriteString(countsNote(before, r.entryCount(args.NotebookID)))
			if len(added) > 0 {
				fmt.Fprintf(&b, "\n**Added:** %s", strings.Join(added, ", "))
			}
			if len(skipped) > 0 {
				fmt.Fprintf(&b, "\n**Skipped:** %s", strings.Join(skipped, ", "))
			}
			if len(failed) > 0 {
				fmt.Fprintf(&b, "\n**Failed:** %s", strings.Join(failed, ", "))
			}
			return msg(b.String())
		}

		// Only kind="ref" points at a message ID (room_ref/query_ref point at a
		// room, task has no ref_id) — resolve a possibly-truncated prefix.
		if kind == "ref" {
			if err := r.resolveInto(&args.RefID); err != nil {
				return msg(fmt.Sprintf("Error: %s", err.Error()))
			}
		}
		entryID, err := r.Server.AddOutlineEntry(args.NotebookID, kind, args.RefID, args.Prose, args.AfterEntryID)
		if err != nil {
			var dup *council.ErrAlreadyReferenced
			if errors.As(err, &dup) {
				return msg(fmt.Sprintf("Already referenced (no-op) — %s '%s' is entry %s in notebook '%s'. Not re-added.", dup.Kind, dup.RefID, dup.EntryID, args.NotebookID))
			}
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		out := fmt.Sprintf("Entry %s added to notebook '%s' (kind: %s).%s", entryID, args.NotebookID, kind, countsNote(before, r.entryCount(args.NotebookID)))
		if kind == "task" {
			if completionMarkerRe.MatchString(args.Prose) {
				out += " Note: add does not close anything — a new task starts open even though its label reads as finished."
			}
			out += fmt.Sprintf(" Close later with action=check, entry_id=%s.", entryID)
		}
		return msg(out)

	case "update":
		if args.EntryID == "" {
			return msg("Error: entry_id is required.")
		}
		if err := validateSize("prose", args.Prose, maxContentLen); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		n := r.entryCount(r.notebookOf(args.NotebookID, args.EntryID))
		if err := r.Server.UpdateOutlineEntry(args.EntryID, args.Prose); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		return msg(fmt.Sprintf("Entry %s updated.%s", args.EntryID, countsNote(n, n)))

	case "start", "check", "uncheck":
		if args.EntryID == "" {
			return msg("Error: entry_id is required.")
		}
		status := map[string]string{"start": "doing", "check": "done", "uncheck": "open"}[args.Action]
		n := r.entryCount(r.notebookOf(args.NotebookID, args.EntryID))
		if err := r.Server.SetTaskStatus(args.EntryID, status); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		label := map[string]string{"doing": "in progress 🔄", "done": "done ☑", "open": "open ☐"}[status]
		return msg(fmt.Sprintf("Task %s marked %s.%s", args.EntryID, label, countsNote(n, n)))

	case "remove":
		if args.EntryID == "" {
			return msg("Error: entry_id is required.")
		}
		nb := r.notebookOf(args.NotebookID, args.EntryID)
		before := r.entryCount(nb)
		if err := r.Server.RemoveOutlineEntry(args.EntryID); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		return msg(fmt.Sprintf("Entry %s removed.%s", args.EntryID, countsNote(before, r.entryCount(nb))))

	case "move":
		if args.EntryID == "" {
			return msg("Error: entry_id is required.")
		}
		n := r.entryCount(r.notebookOf(args.NotebookID, args.EntryID))
		if err := r.Server.MoveOutlineEntry(args.EntryID, args.AfterEntryID); err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		if args.AfterEntryID == "" {
			return msg(fmt.Sprintf("Entry %s moved to the top.%s", args.EntryID, countsNote(n, n)))
		}
		return msg(fmt.Sprintf("Entry %s moved after %s.%s", args.EntryID, args.AfterEntryID, countsNote(n, n)))

	default:
		return msg(fmt.Sprintf("Error: unknown action '%s'. Valid actions: create, add, update, start, check, uncheck, remove, move, delete, restore, purge.", args.Action))
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
// statusWanted reports whether a work item in the given state should render
// under the requested status filter. An empty filter keeps everything, which
// is the pre-existing behaviour.
func statusWanted(filter, state string) bool {
	return filter == "" || filter == state
}

func (r *Registry) renderOutline(notebookID string, level int, f council.OutlineFilter) (*mcp.CallToolResult, ToolOutput, error) {
	text, err := r.outlineText(notebookID, level, f)
	if err != nil {
		return textResult(fmt.Sprintf("Error: %s", err.Error()))
	}
	return textResult(text)
}

// outlineText renders a notebook outline as markdown (also used for the
// pre-delete snapshot). f narrows the work items shown; see council.OutlineFilter.
func (r *Registry) outlineText(notebookID string, level int, f council.OutlineFilter) (string, error) {
	status := f.Status
	notebook, entries, err := r.Server.GetOutlineFiltered(notebookID, f)
	if err != nil {
		return "", err
	}
	filtered := f.Status != "" || f.Project != "" || f.EntryID != "" || f.Limit > 0

	outlineIDs := newRoomIDCache(r.Server)
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
	fmt.Fprintf(&b, "**Notebook:** %s | **Project:** %s | **Entries:** %d", notebook.ID, scope, notebook.EntryCount)
	if filtered {
		fmt.Fprintf(&b, " | **Showing:** %d", len(entries))
	}
	b.WriteString("\n---\n")

	if len(entries) == 0 {
		if filtered && notebook.EntryCount > 0 {
			b.WriteString("\nNo entries match the filter (status/project/entry_id). Drop a filter to see the rest.\n")
			return b.String(), nil
		}
		fmt.Fprintf(&b, "\nEmpty notebook. Add entries with edit_notebook(action=add, notebook_id=%s, ref_id=<message_id> or prose=<markdown>).\n", notebook.ID)
		return b.String(), nil
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
			fmt.Fprintf(&b, "\n> **[#%s %s] [%s] %s (%s)%s:**\n> %s\n*(entry %s)*\n",
				outlineIDs.Of(e.RefRoomID, e.RefID), ts, e.RefRoomID, e.RefAuthor, e.RefType, pin,
				strings.ReplaceAll(content, "\n", "\n> "), e.ID)
		}
	}

	if len(doingTasks) > 0 && statusWanted(status, "doing") {
		fmt.Fprintf(&b, "\n## 🔄 In progress (%d)\n", len(doingTasks))
		for _, e := range doingTasks {
			renderTask(&b, e, level)
		}
	}
	if len(openTasks) > 0 && statusWanted(status, "open") {
		fmt.Fprintf(&b, "\n## ☐ Open (%d)\n", len(openTasks))
		for _, e := range openTasks {
			renderTask(&b, e, level)
		}
	}
	if len(doneTasks) > 0 && statusWanted(status, "done") {
		fmt.Fprintf(&b, "\n## ☑ Done (%d)\n", len(doneTasks))
		for _, e := range doneTasks {
			renderTask(&b, e, level)
		}
	}
	if len(inFlight) > 0 && statusWanted(status, "open") {
		fmt.Fprintf(&b, "\n## 🔄 In flight (%d)\n", len(inFlight))
		for _, e := range inFlight {
			renderRoomRef(&b, e, level)
		}
	}
	if len(doneRooms) > 0 && statusWanted(status, "done") {
		fmt.Fprintf(&b, "\n## ✅ Done (%d)\n", len(doneRooms))
		for _, e := range doneRooms {
			renderRoomRef(&b, e, level)
		}
	}

	return b.String(), nil
}

// roomRefDone reports whether a transcluded room_ref counts as finished work.
// A resolved/archived room is Done; everything else (active, paused, or a
// dangling ref that needs attention) is still In flight.
func roomRefDone(e council.OutlineEntry) bool {
	return e.RefFound && (e.RefStatus == "resolved" || e.RefStatus == "archived")
}

// renderRoomRef writes one work-list item: the room's live status, topic, and
// latest decision/action. A dangling room_ref renders as a warning in place.
func renderRoomRef(b *strings.Builder, e council.OutlineEntry, level int) {
	if !e.RefFound {
		fmt.Fprintf(b, "\n⚠ **referenced room '%.12s' not found** — deleted, or it lives on another cluster node. *(entry %s)*\n", e.RefID, e.ID)
		return
	}
	topic := e.RefTopic
	if level > 0 {
		topic = clipLabel(topic)
	}
	fmt.Fprintf(b, "\n**[%s] %s** — %s\n", e.RefStatus, e.RefID, topic)
	// At any level above 0 the room's status, id and topic are the structure;
	// the latest-message excerpt is the body, so it collapses away like a
	// transcluded message body does.
	if e.RefContent != "" && level == 0 {
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
func renderTask(b *strings.Builder, e council.OutlineEntry, level int) {
	box := "[ ]"
	switch e.Status {
	case "done":
		box = "[x]"
	case "doing":
		box = "[~]"
	}
	label := strings.ReplaceAll(strings.TrimSpace(e.Prose), "\n", " ")
	if level > 0 {
		label = clipLabel(label)
	}
	fmt.Fprintf(b, "- %s %s *(entry %s)*\n", box, label, e.ID)
}

// clipLabel shortens a work-item label to its first sentence, or to
// clipLabelMax characters, whichever comes first. A task label is nominally a
// one-liner, but in practice they accumulate into incident write-ups of a
// thousand characters or more — which is why `level` used to save almost
// nothing on a task-heavy notebook: it clipped prose and transclusions while
// tasks and room_refs, the actual bulk, always rendered in full.
func clipLabel(s string) string {
	if i := strings.Index(s, ". "); i > 0 && i+1 < clipLabelMax {
		return s[:i+1] + " …"
	}
	if len(s) > clipLabelMax {
		cut := clipLabelMax
		if j := strings.LastIndex(s[:clipLabelMax], " "); j > clipLabelMax/2 {
			cut = j
		}
		return strings.TrimRight(s[:cut], " ,;:") + " …"
	}
	return s
}

const clipLabelMax = 140

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
