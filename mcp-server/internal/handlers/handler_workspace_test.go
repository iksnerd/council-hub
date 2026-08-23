package handlers

import (
	"context"
	"strings"
	"testing"
)

// postFrom posts a message declaring a working tree, returning the response text.
func postFrom(t *testing.T, r *Registry, author, roomID, workspace string) string {
	t.Helper()
	res, _, err := r.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID:      roomID,
		Author:      author,
		Message:     "working",
		MessageType: "note",
		Workspace:   workspace,
	})
	if err != nil {
		t.Fatalf("post as %s: %v", author, err)
	}
	return resultText(res)
}

func TestPostToRoomWarnsOnSharedCheckout(t *testing.T) {
	r := setupHandlerTest(t)
	if err := r.Server.CreateRoom("room-a", "", "", "", "", "", ""); err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	// First poster is alone in the tree — nothing to warn about.
	if got := postFrom(t, r, "codex", "room-a", "/work/repo"); strings.Contains(got, "Shared working tree") {
		t.Errorf("first poster should see no warning:\n%s", got)
	}

	// Second poster shares it.
	got := postFrom(t, r, "claude", "room-a", "/work/repo")
	if !strings.Contains(got, "Shared working tree") {
		t.Fatalf("expected a shared-tree warning:\n%s", got)
	}
	for _, want := range []string{"codex", "/work/repo", "git add -A"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning missing %q:\n%s", want, got)
		}
	}
	// The post itself must still succeed — this is advisory, not a gate.
	if !strings.Contains(got, "posted to room 'room-a'") {
		t.Errorf("post should still succeed:\n%s", got)
	}
}

func TestPostToRoomNoWorkspaceIsUnchanged(t *testing.T) {
	r := setupHandlerTest(t)
	if err := r.Server.CreateRoom("room-a", "", "", "", "", "", ""); err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	// Two agents, neither declaring a tree: no rows, no warning, no behaviour change
	// for every caller that never passes the parameter.
	postFrom(t, r, "codex", "room-a", "")
	got := postFrom(t, r, "claude", "room-a", "")
	if strings.Contains(got, "Shared working tree") {
		t.Errorf("undeclared workspaces must not warn:\n%s", got)
	}

	var n int
	if err := r.Server.DB.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("expected no workspace rows, got %d", n)
	}
}

func TestPostToRoomRejectsOversizedWorkspace(t *testing.T) {
	r := setupHandlerTest(t)
	if err := r.Server.CreateRoom("room-a", "", "", "", "", "", ""); err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	res, _, err := r.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "room-a", Author: "claude", Message: "hi", MessageType: "note",
		Workspace: "/" + strings.Repeat("a", maxWorkspaceLen),
	})
	if err != nil {
		t.Fatalf("handlePostToRoom: %v", err)
	}
	if got := resultText(res); !strings.Contains(got, "Error") {
		t.Errorf("expected a size error, got:\n%s", got)
	}
}

func TestReadRoomSurfacesSharedCheckoutAcrossRooms(t *testing.T) {
	r := setupHandlerTest(t)
	for _, id := range []string{"room-a", "room-b"} {
		if err := r.Server.CreateRoom(id, "", "", "", "", "", ""); err != nil {
			t.Fatalf("CreateRoom %s: %v", id, err)
		}
	}

	// The two agents share a tree but talk in different rooms. They still share a
	// git index, so reading either room must surface it.
	postFrom(t, r, "claude", "room-a", "/work/repo")
	postFrom(t, r, "codex", "room-b", "/work/repo")

	res, _, err := r.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "room-a"})
	if err != nil {
		t.Fatalf("handleReadRoom: %v", err)
	}
	got := resultText(res)
	if !strings.Contains(got, "Shared working tree") {
		t.Fatalf("expected read_room to surface the shared tree:\n%s", got)
	}
	for _, want := range []string{"claude", "codex", "/work/repo"} {
		if !strings.Contains(got, want) {
			t.Errorf("read_room note missing %q:\n%s", want, got)
		}
	}
}

func TestReadRoomQuietWhenNoTreeIsShared(t *testing.T) {
	r := setupHandlerTest(t)
	if err := r.Server.CreateRoom("room-a", "", "", "", "", "", ""); err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	postFrom(t, r, "claude", "room-a", "/work/repo")

	res, _, err := r.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "room-a"})
	if err != nil {
		t.Fatalf("handleReadRoom: %v", err)
	}
	if got := resultText(res); strings.Contains(got, "Shared working tree") {
		t.Errorf("a solo tree must not warn:\n%s", got)
	}
}
