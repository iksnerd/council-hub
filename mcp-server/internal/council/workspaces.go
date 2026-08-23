package council

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// SharedWorkspaceWindow bounds how recently another participant must have posted
// from a working tree for a collision to still be worth reporting. Past it the
// other session has almost certainly ended, and a warning about a tree nobody is
// in any more is noise that trains readers to ignore the real ones.
const SharedWorkspaceWindow = 24 * time.Hour

// WorkspacePeer is another participant last seen working in the same tree.
type WorkspacePeer struct {
	Author   string
	RoomID   string
	LastSeen time.Time
}

// NormalizeWorkspace canonicalizes a client-supplied working-tree path so that
// "/repo", "/repo/" and "/repo/sub/.." all collide. The path is treated as an
// opaque identifier, never touched on disk: it describes the *client's*
// filesystem, and the server may be in a container that cannot see it, so
// symlink resolution and existence checks are neither possible nor meaningful.
func NormalizeWorkspace(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}

// RecordWorkspace notes that author is working in workspace, refreshing the
// last-seen stamp on every post. Empty author or workspace is a no-op rather
// than an error: declaring a workspace is optional, and a caller that omits it
// should simply get no detection, not a failed write.
func (s *Server) RecordWorkspace(author, roomID, workspace string) error {
	workspace = NormalizeWorkspace(workspace)
	if author == "" || workspace == "" {
		return nil
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	_, err := s.DB.Exec(`
		INSERT INTO workspaces (author, workspace, room_id, last_seen)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(author, workspace, room_id) DO UPDATE SET
			last_seen = CURRENT_TIMESTAMP
	`, author, workspace, roomID)
	return err
}

// SharedWorkspacePeers returns every OTHER author seen in workspace within
// SharedWorkspaceWindow, newest first.
//
// The scan is deliberately not scoped to a room: two agents sharing a checkout
// collide whether or not they are talking in the same room, and the git index
// they share does not care. It is equally deliberately node-local — a shared
// working tree implies a shared filesystem, so participants on two cluster nodes
// are on two machines and cannot be in one tree. Node-local is the correct scope
// here, not a limitation.
//
// The match is on the literal normalized path, which can in principle collide
// across two machines that happen to use the same layout. That is accepted: a
// false warning costs a moment's attention, a missed one costs a clobbered
// commit, and those are not comparable.
func (s *Server) SharedWorkspacePeers(author, workspace string) ([]WorkspacePeer, error) {
	workspace = NormalizeWorkspace(workspace)
	if workspace == "" {
		return nil, nil
	}

	s.Mu.RLock()
	defer s.Mu.RUnlock()

	// One source of truth for the window: derive SQLite's modifier from the Go
	// constant rather than writing "-24 hours" twice and letting them drift.
	// Ordered, then deduped in Go rather than grouped in SQL: an aggregate like
	// MAX(last_seen) loses the column's DATETIME affinity, and the driver hands back
	// a string instead of a time.Time. Newest-first ordering means the first row per
	// author is the one to keep.
	rows, err := s.DB.Query(`
		SELECT author, room_id, last_seen FROM workspaces
		WHERE workspace = ? AND author != ? AND last_seen >= datetime('now', ?)
		ORDER BY last_seen DESC
	`, workspace, author, workspaceCutoff())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	// One agent posting from one tree into three rooms is one collision, not three.
	var peers []WorkspacePeer
	seen := map[string]bool{}
	for rows.Next() {
		var p WorkspacePeer
		if err := rows.Scan(&p.Author, &p.RoomID, &p.LastSeen); err != nil {
			return nil, err
		}
		if seen[p.Author] {
			continue
		}
		seen[p.Author] = true
		peers = append(peers, p)
	}
	return peers, rows.Err()
}

// WorkspaceGroup is one working tree and every participant currently in it.
type WorkspaceGroup struct {
	Workspace string
	Peers     []WorkspacePeer
}

// SharedWorkspacesInRoom returns the working trees touched by this room that hold
// more than one participant — the read-side half of the detection, so a reader who
// did not post still sees the hazard. Participants are counted across every room,
// not just this one: two agents share a git index whether or not they are talking
// in the same place.
func (s *Server) SharedWorkspacesInRoom(roomID string) ([]WorkspaceGroup, error) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	cutoff := workspaceCutoff()
	rows, err := s.DB.Query(`
		SELECT workspace, author, room_id, last_seen FROM workspaces
		WHERE last_seen >= datetime('now', ?)
		  AND workspace IN (
			SELECT workspace FROM workspaces
			WHERE room_id = ? AND last_seen >= datetime('now', ?)
		  )
		ORDER BY workspace, last_seen DESC
	`, cutoff, roomID, cutoff)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	// Same reason as above: dedup per (workspace, author) in Go rather than with an
	// aggregate, so last_seen keeps its type. Rows arrive grouped by workspace and
	// newest-first within it.
	var groups []WorkspaceGroup
	seen := map[string]bool{}
	for rows.Next() {
		var ws string
		var p WorkspacePeer
		if err := rows.Scan(&ws, &p.Author, &p.RoomID, &p.LastSeen); err != nil {
			return nil, err
		}
		key := ws + "\x00" + p.Author
		if seen[key] {
			continue
		}
		seen[key] = true
		if n := len(groups); n > 0 && groups[n-1].Workspace == ws {
			groups[n-1].Peers = append(groups[n-1].Peers, p)
			continue
		}
		groups = append(groups, WorkspaceGroup{Workspace: ws, Peers: []WorkspacePeer{p}})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// A tree with one occupant is the normal case and not worth reporting.
	shared := groups[:0]
	for _, g := range groups {
		if len(g.Peers) > 1 {
			shared = append(shared, g)
		}
	}
	return shared, nil
}

// workspaceCutoff renders SharedWorkspaceWindow as a SQLite datetime modifier, so
// the window has one source of truth instead of a Go constant and a hand-written
// "-24 hours" that drift apart.
func workspaceCutoff() string {
	return fmt.Sprintf("-%d seconds", int(SharedWorkspaceWindow.Seconds()))
}

// SharedWorkspaceWarning renders the peers sharing a working tree, or "" when
// there are none. It names the concrete failure rather than saying "conflict":
// the danger is not two agents editing files, which is safe when the files are
// disjoint, but the one piece of state they unavoidably share.
func SharedWorkspaceWarning(workspace string, peers []WorkspacePeer) string {
	if len(peers) == 0 {
		return ""
	}

	who := make([]string, 0, len(peers))
	for _, p := range peers {
		if p.RoomID != "" {
			who = append(who, fmt.Sprintf("%s (%s, in %s)", p.Author, humanSince(p.LastSeen), p.RoomID))
			continue
		}
		who = append(who, fmt.Sprintf("%s (%s)", p.Author, humanSince(p.LastSeen)))
	}

	return fmt.Sprintf("\n\n⚠️ **Shared working tree** — `%s` is also in use by %s. "+
		"Disjoint file edits are safe; the git index is not: `git add -A` from either session "+
		"stages the other's in-progress work. Stage explicit paths instead.",
		workspace, strings.Join(who, ", "))
}

// SharedWorkspacesNote renders the shared trees for a room view, or "" when none
// are shared. Read-side counterpart to SharedWorkspaceWarning, which speaks to the
// agent that just posted; this one speaks to whoever is reading the room.
func SharedWorkspacesNote(groups []WorkspaceGroup) string {
	if len(groups) == 0 {
		return ""
	}

	var b strings.Builder
	for _, g := range groups {
		who := make([]string, 0, len(g.Peers))
		for _, p := range g.Peers {
			who = append(who, fmt.Sprintf("%s (%s)", p.Author, humanSince(p.LastSeen)))
		}
		fmt.Fprintf(&b, "\n⚠️ **Shared working tree:** `%s` — %s. "+
			"Disjoint file edits are safe; the git index is not: `git add -A` from any of them "+
			"stages the others' in-progress work.\n", g.Workspace, strings.Join(who, ", "))
	}
	return b.String()
}

// humanSince renders a coarse "how long ago", enough to judge whether the other
// participant is plausibly still working. Precision past the minute would imply
// a liveness guarantee this has no way to make — the stamp is the last post, not
// a heartbeat.
func humanSince(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
