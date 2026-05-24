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

func TestHandleReadRoomIncludeLastN(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-lastn")
	mustPost(t, reg.Server, "h-lastn", "Claude", "Message one")
	mustPost(t, reg.Server, "h-lastn", "Gemini", "Message two")

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{
		RoomID:       "h-lastn",
		IncludeLastN: "2",
	})
	text := resultText(res)
	if !strings.Contains(text, "Recent messages") {
		t.Errorf("expected recent messages section, got: %s", text)
	}
	if !strings.Contains(text, "Message one") || !strings.Contains(text, "Message two") {
		t.Errorf("expected message content, got: %s", text)
	}
}

func TestHandleReadRoomIncludeLastNZero(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-lastn-zero")
	mustPost(t, reg.Server, "h-lastn-zero", "Claude", "Hidden message")

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{
		RoomID:       "h-lastn-zero",
		IncludeLastN: "0",
	})
	text := resultText(res)
	if strings.Contains(text, "Recent messages") {
		t.Errorf("expected no messages section for n=0, got: %s", text)
	}
}

func TestHandleReadRoomIncludeLastNClamp(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-lastn-clamp")
	mustPost(t, reg.Server, "h-lastn-clamp", "Claude", "Clamped message")

	// Request 999 — should be clamped to 50 max without error
	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{
		RoomID:       "h-lastn-clamp",
		IncludeLastN: "999",
	})
	text := resultText(res)
	if !strings.Contains(text, "Recent messages") {
		t.Errorf("expected recent messages section for clamped n, got: %s", text)
	}
}

func TestHandleReadRoomIncludeLastNInvalid(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-lastn-invalid")
	mustPost(t, reg.Server, "h-lastn-invalid", "Claude", "Message")

	// Non-numeric string — Sscanf sets n=0, should not include messages
	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{
		RoomID:       "h-lastn-invalid",
		IncludeLastN: "abc",
	})
	text := resultText(res)
	if strings.Contains(text, "Recent messages") {
		t.Errorf("expected no messages section for invalid n, got: %s", text)
	}
}

func TestHandleReadRoomIncludeLastNNoMessages(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-lastn-empty")

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{
		RoomID:       "h-lastn-empty",
		IncludeLastN: "5",
	})
	text := resultText(res)
	// No messages in room — section should not appear
	if strings.Contains(text, "Recent messages") {
		t.Errorf("expected no messages section for empty room, got: %s", text)
	}
}

func TestHandleReadRoomIncludeRelatedSummaries(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-main-room", withRelatedRooms("h-related-room"))
	mustCreateRoom(t, reg.Server, "h-related-room", withDescription("Related room topic"), withSystemPrompt("Related prompt"))

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{
		RoomID:                  "h-main-room",
		IncludeRelatedSummaries: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "h-related-room") {
		t.Errorf("expected related room ID in output, got: %s", text)
	}
	if !strings.Contains(text, "Related room topic") {
		t.Errorf("expected related room topic, got: %s", text)
	}
}

func TestHandleReadRoomIncludeRelatedSummariesNotFound(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-main-missing-rel", withRelatedRooms("ghost-related"))

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{
		RoomID:                  "h-main-missing-rel",
		IncludeRelatedSummaries: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "ghost-related") {
		t.Errorf("expected ghost-related in output, got: %s", text)
	}
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found message for missing related room, got: %s", text)
	}
}

func TestHandleReadRoomIncludeRelatedSummariesWithPinned(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-main-pinned-rel", withRelatedRooms("h-pinned-related"))
	mustCreateRoom(t, reg.Server, "h-pinned-related")
	msgID := mustPost(t, reg.Server, "h-pinned-related", "Claude", "Pinned excerpt content")
	if _, err := reg.Server.PinMessage("h-pinned-related", msgID); err != nil {
		t.Fatalf("PinMessage error: %v", err)
	}

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{
		RoomID:                  "h-main-pinned-rel",
		IncludeRelatedSummaries: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "Pinned excerpt content") {
		t.Errorf("expected pinned message excerpt, got: %s", text)
	}
}

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

func TestHandleGetConceptMapInferFromProject(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "cm-root", withProject("alpha"))
	mustCreateRoom(t, reg.Server, "cm-sibling", withProject("alpha"))
	mustCreateRoom(t, reg.Server, "cm-other", withProject("beta"))

	res, _, err := reg.handleGetConceptMap(context.Background(), nil, GetConceptMapInput{
		RoomID:    "cm-root",
		InferFrom: "project",
	})
	if err != nil {
		t.Fatalf("handleGetConceptMap error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "cm-sibling") {
		t.Errorf("expected same-project room 'cm-sibling' in output: %s", text)
	}
	if strings.Contains(text, "cm-other") {
		t.Errorf("different-project room 'cm-other' should not appear: %s", text)
	}
	if !strings.Contains(text, "inferred: project") {
		t.Errorf("expected inferred annotation in output: %s", text)
	}
	if !strings.Contains(text, "infer_from: project") {
		t.Errorf("expected infer_from header in output: %s", text)
	}
}

func TestHandleGetConceptMapInferFromTags(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "cmt-root", withTags("go,api"))
	mustCreateRoom(t, reg.Server, "cmt-tagged", withTags("go,grpc"))
	mustCreateRoom(t, reg.Server, "cmt-unrelated", withTags("python"))

	res, _, err := reg.handleGetConceptMap(context.Background(), nil, GetConceptMapInput{
		RoomID:    "cmt-root",
		InferFrom: "tags",
	})
	if err != nil {
		t.Fatalf("handleGetConceptMap error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "cmt-tagged") {
		t.Errorf("expected shared-tag room 'cmt-tagged' in output: %s", text)
	}
	if strings.Contains(text, "cmt-unrelated") {
		t.Errorf("no-shared-tag room 'cmt-unrelated' should not appear: %s", text)
	}
	if !strings.Contains(text, "inferred: tags") {
		t.Errorf("expected inferred tags annotation: %s", text)
	}
}
