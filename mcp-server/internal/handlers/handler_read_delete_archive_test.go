package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== read_room ==========

func TestHandleReadRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-read", withDescription("Read test"), withProject("proj"), withTechStack("Go"), withTags("tag1"), withSystemPrompt("prompt"), withRelatedRooms("related-a"))

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "h-read"})
	text := resultText(res)
	if !strings.Contains(text, "Read test") {
		t.Error("missing topic")
	}
	if !strings.Contains(text, "proj") {
		t.Error("missing project")
	}
	if !strings.Contains(text, "Related Rooms:** related-a") {
		t.Error("missing related rooms")
	}
}

func TestHandleReadRoomMissing(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleReadRoomNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found, got: %s", text)
	}
}

func TestHandleReadRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "hdb-room"})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found, got: %s", text)
	}
}

// ========== delete_room ==========

func TestHandleDeleteRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-del")

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "h-del"})
	text := resultText(res)
	if !strings.Contains(text, "permanently deleted") {
		t.Errorf("expected deleted, got: %s", text)
	}
}

func TestHandleDeleteRoomMissing(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing room_id")
	}
}

func TestHandleDeleteRoomNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleDeleteRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "hdb-room"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleDeleteRoomCascadeClean(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-cascade-a")
	mustCreateRoom(t, reg.Server, "h-cascade-b")
	reg.Server.UpdateRoom("h-cascade-b", "", "", "", "", "", "", "", "h-cascade-a")

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "h-cascade-a"})
	if !strings.Contains(resultText(res), "permanently deleted") {
		t.Fatalf("expected deleted confirmation, got: %s", resultText(res))
	}

	roomB, err := reg.Server.GetRoom("h-cascade-b")
	if err != nil {
		t.Fatalf("GetRoom failed: %v", err)
	}
	if strings.Contains(roomB.RelatedRooms, "h-cascade-a") {
		t.Errorf("h-cascade-b still references deleted h-cascade-a: %q", roomB.RelatedRooms)
	}
}

// ========== archive_room ==========

func TestHandleArchiveRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-archive")
	mustPost(t, reg.Server, "h-archive", "Claude", "Archive me")

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "h-archive"})
	text := resultText(res)
	if !strings.Contains(text, "archived") {
		t.Errorf("expected archived, got: %s", text)
	}
}

func TestHandleArchiveRoomAndDelete(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-archive-del")
	mustPost(t, reg.Server, "h-archive-del", "Claude", "Gone")

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{
		RoomID: "h-archive-del", Delete: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "deleted") {
		t.Errorf("expected deleted, got: %s", text)
	}

	_, err := reg.Server.GetRoom("h-archive-del")
	if err == nil {
		t.Error("room should be deleted after archive+delete")
	}
}

func TestHandleArchiveRoomMissing(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error")
	}
}

func TestHandleArchiveRoomNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleArchiveRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "hdb-room"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// ========== list_archives ==========

func TestHandleListArchivesEmpty(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)

	res, _, _ := reg.handleListArchives(context.Background(), nil, ListArchivesInput{})
	if !strings.Contains(resultText(res), "No archives found") {
		t.Error("expected no archives message")
	}
}

func TestHandleListArchives(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)
	mustCreateRoom(t, reg.Server, "h-list-arch-a")
	mustPost(t, reg.Server, "h-list-arch-a", "Claude", "Message A")
	mustCreateRoom(t, reg.Server, "h-list-arch-b")
	mustPost(t, reg.Server, "h-list-arch-b", "Claude", "Message B")

	reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "h-list-arch-a"}) //nolint:errcheck
	reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "h-list-arch-b"}) //nolint:errcheck

	res, _, _ := reg.handleListArchives(context.Background(), nil, ListArchivesInput{})
	text := resultText(res)
	if !strings.Contains(text, "h-list-arch-a") {
		t.Error("expected h-list-arch-a in listing")
	}
	if !strings.Contains(text, "h-list-arch-b") {
		t.Error("expected h-list-arch-b in listing")
	}
	if !strings.Contains(text, "Found 2 archive") {
		t.Errorf("expected count in header, got: %s", text)
	}
}

// ========== read_archive ==========

func TestHandleReadArchive(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)
	mustCreateRoom(t, reg.Server, "h-read-arch")
	mustPost(t, reg.Server, "h-read-arch", "Claude", "Archived content here")

	reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "h-read-arch"}) //nolint:errcheck

	res, _, _ := reg.handleReadArchive(context.Background(), nil, ReadArchiveInput{RoomID: "h-read-arch"})
	text := resultText(res)
	if !strings.Contains(text, "h-read-arch") {
		t.Error("expected room ID in archive content")
	}
	if !strings.Contains(text, "Archived content here") {
		t.Error("expected message content in archive")
	}
}

func TestHandleReadArchiveNotFound(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)

	res, _, _ := reg.handleReadArchive(context.Background(), nil, ReadArchiveInput{RoomID: "ghost-archive"})
	if !strings.Contains(resultText(res), "not found") {
		t.Error("expected not found error")
	}
}

func TestHandleReadArchiveMissingID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReadArchive(context.Background(), nil, ReadArchiveInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing room_id")
	}
}

// ========== get_concept_map ==========

func TestHandleGetConceptMap(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "root-node", withDescription("Root of graph"), withTags("tag0"))
	mustCreateRoom(t, reg.Server, "child-node", withDescription("Direct child"), withTags("tag1"), withRelatedRooms("root-node"))

	// Test basic traversal
	res, _, err := reg.handleGetConceptMap(context.Background(), nil, GetConceptMapInput{
		RoomID: "root-node",
	})
	if err != nil {
		t.Fatalf("handleGetConceptMap error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "Concept Map: root-node") {
		t.Errorf("missing header: %s", text)
	}
	if !strings.Contains(text, "root-node") || !strings.Contains(text, "child-node") {
		t.Errorf("missing nodes: %s", text)
	}
	if !strings.Contains(text, "tag0") || !strings.Contains(text, "tag1") {
		t.Errorf("missing tags: %s", text)
	}
	if !strings.Contains(text, "via: root-node") {
		t.Errorf("missing link info: %s", text)
	}

	// Test depth limiting
	res, _, _ = reg.handleGetConceptMap(context.Background(), nil, GetConceptMapInput{
		RoomID:   "root-node",
		MaxDepth: "0",
	})
	text = resultText(res)
	if strings.Contains(text, "child-node") {
		t.Errorf("depth 0 should only return root, got: %s", text)
	}
}

func TestHandleGetConceptMapMissingID(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleGetConceptMap(context.Background(), nil, GetConceptMapInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing room_id")
	}
}

func TestHandleGetConceptMapNotFound(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleGetConceptMap(context.Background(), nil, GetConceptMapInput{RoomID: "ghost"})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for nonexistent room")
	}
}
