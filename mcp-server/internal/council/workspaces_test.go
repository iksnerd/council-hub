package council

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeWorkspace(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/repo", "/repo"},
		{"/repo/", "/repo"},
		{"  /repo  ", "/repo"},
		{"/repo/sub/..", "/repo"},
		{"/repo//sub", "/repo/sub"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := NormalizeWorkspace(c.in); got != c.want {
			t.Errorf("NormalizeWorkspace(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSharedWorkspacePeersDetectsCollision(t *testing.T) {
	s := setupTestServer(t)

	if err := s.RecordWorkspace("codex", "room-a", "/work/repo"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}
	// Trailing slash must still collide — the whole point of normalizing.
	if err := s.RecordWorkspace("claude", "room-b", "/work/repo/"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}

	peers, err := s.SharedWorkspacePeers("claude", "/work/repo")
	if err != nil {
		t.Fatalf("SharedWorkspacePeers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d: %+v", len(peers), peers)
	}
	if peers[0].Author != "codex" {
		t.Errorf("peer author = %q, want codex", peers[0].Author)
	}
	if peers[0].RoomID != "room-a" {
		t.Errorf("peer room = %q, want room-a", peers[0].RoomID)
	}
}

func TestSharedWorkspacePeersExcludesSelf(t *testing.T) {
	s := setupTestServer(t)

	// The same agent posting from one tree into three rooms is not a collision.
	for _, room := range []string{"room-a", "room-b", "room-c"} {
		if err := s.RecordWorkspace("claude", room, "/work/repo"); err != nil {
			t.Fatalf("RecordWorkspace: %v", err)
		}
	}

	peers, err := s.SharedWorkspacePeers("claude", "/work/repo")
	if err != nil {
		t.Fatalf("SharedWorkspacePeers: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected no peers, got %d: %+v", len(peers), peers)
	}
}

func TestSharedWorkspacePeersCollapsesOneRowPerAuthor(t *testing.T) {
	s := setupTestServer(t)

	if err := s.RecordWorkspace("claude", "room-a", "/work/repo"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}
	// One peer in three rooms is one collision, not three.
	for _, room := range []string{"room-a", "room-b", "room-c"} {
		if err := s.RecordWorkspace("codex", room, "/work/repo"); err != nil {
			t.Fatalf("RecordWorkspace: %v", err)
		}
	}

	peers, err := s.SharedWorkspacePeers("claude", "/work/repo")
	if err != nil {
		t.Fatalf("SharedWorkspacePeers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d: %+v", len(peers), peers)
	}
}

func TestSharedWorkspacePeersIgnoresDifferentTrees(t *testing.T) {
	s := setupTestServer(t)

	if err := s.RecordWorkspace("codex", "room-a", "/work/other"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}
	peers, err := s.SharedWorkspacePeers("claude", "/work/repo")
	if err != nil {
		t.Fatalf("SharedWorkspacePeers: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected no peers across different trees, got %+v", peers)
	}
}

func TestSharedWorkspacePeersHonorsWindow(t *testing.T) {
	s := setupTestServer(t)

	if err := s.RecordWorkspace("codex", "room-a", "/work/repo"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}
	// Age the peer past the window. A tree nobody is in any more is not a hazard,
	// and warning about it trains readers to ignore the real ones.
	stale := time.Now().UTC().Add(-SharedWorkspaceWindow - time.Hour)
	if _, err := s.DB.Exec(
		`UPDATE workspaces SET last_seen = ? WHERE author = 'codex'`,
		stale.Format("2006-01-02 15:04:05"),
	); err != nil {
		t.Fatalf("age peer: %v", err)
	}

	peers, err := s.SharedWorkspacePeers("claude", "/work/repo")
	if err != nil {
		t.Fatalf("SharedWorkspacePeers: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected stale peer to be filtered, got %+v", peers)
	}
}

func TestRecordWorkspaceIgnoresEmptyInput(t *testing.T) {
	s := setupTestServer(t)

	// Declaring a workspace is optional: omitting it means no detection, not a
	// failed write.
	if err := s.RecordWorkspace("claude", "room-a", ""); err != nil {
		t.Fatalf("empty workspace should be a no-op, got %v", err)
	}
	if err := s.RecordWorkspace("", "room-a", "/work/repo"); err != nil {
		t.Fatalf("empty author should be a no-op, got %v", err)
	}

	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("expected no rows written, got %d", n)
	}
}

func TestSharedWorkspacesInRoom(t *testing.T) {
	s := setupTestServer(t)

	if err := s.RecordWorkspace("claude", "room-a", "/work/repo"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}
	// The peer is in a different room. It still shares the git index, so the room
	// view must surface it — this is the case room-scoping would have missed.
	if err := s.RecordWorkspace("codex", "room-b", "/work/repo"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}
	// An unrelated tree with a single occupant must not appear.
	if err := s.RecordWorkspace("gemini", "room-a", "/work/solo"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}

	groups, err := s.SharedWorkspacesInRoom("room-a")
	if err != nil {
		t.Fatalf("SharedWorkspacesInRoom: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 shared tree, got %d: %+v", len(groups), groups)
	}
	if groups[0].Workspace != "/work/repo" {
		t.Errorf("workspace = %q, want /work/repo", groups[0].Workspace)
	}
	if len(groups[0].Peers) != 2 {
		t.Errorf("expected 2 participants, got %d: %+v", len(groups[0].Peers), groups[0].Peers)
	}
}

func TestSharedWorkspacesInRoomIgnoresUnrelatedTrees(t *testing.T) {
	s := setupTestServer(t)

	// Two agents share a tree, but nobody in room-a is in it.
	if err := s.RecordWorkspace("claude", "room-b", "/work/repo"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}
	if err := s.RecordWorkspace("codex", "room-c", "/work/repo"); err != nil {
		t.Fatalf("RecordWorkspace: %v", err)
	}

	groups, err := s.SharedWorkspacesInRoom("room-a")
	if err != nil {
		t.Fatalf("SharedWorkspacesInRoom: %v", err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected no groups for an uninvolved room, got %+v", groups)
	}
}

func TestSharedWorkspaceWarningRendering(t *testing.T) {
	if got := SharedWorkspaceWarning("/work/repo", nil); got != "" {
		t.Errorf("no peers should render nothing, got %q", got)
	}

	got := SharedWorkspaceWarning("/work/repo", []WorkspacePeer{
		{Author: "codex", RoomID: "room-a", LastSeen: time.Now()},
	})
	for _, want := range []string{"/work/repo", "codex", "room-a", "git add -A"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning missing %q:\n%s", want, got)
		}
	}
}

func TestSharedWorkspacesNoteRendering(t *testing.T) {
	if got := SharedWorkspacesNote(nil); got != "" {
		t.Errorf("no groups should render nothing, got %q", got)
	}

	got := SharedWorkspacesNote([]WorkspaceGroup{{
		Workspace: "/work/repo",
		Peers: []WorkspacePeer{
			{Author: "claude", LastSeen: time.Now()},
			{Author: "codex", LastSeen: time.Now()},
		},
	}})
	for _, want := range []string{"/work/repo", "claude", "codex", "git add -A"} {
		if !strings.Contains(got, want) {
			t.Errorf("note missing %q:\n%s", want, got)
		}
	}
}
