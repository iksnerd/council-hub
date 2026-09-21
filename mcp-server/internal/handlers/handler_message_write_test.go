package handlers

import (
	"context"
	"strings"
	"testing"

	"council-hub/internal/council"
)

// Correcting a long ledger entry used to mean re-transmitting the whole body,
// because update_message replaces content wholesale. That is how a 39-message
// sha remap became 39 hand-transcriptions of records that are supposed to be
// immutable. append builds the new content server-side from the stored one.
func TestUpdateMessageAppendExtendsWithoutRetransmitting(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "app-room")
	posted := mustPost(t, reg.Server, "app-room", "claude", "Original body that the caller must not have to resend.")

	res, _, err := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: posted, Append: "A correction added later.",
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if strings.Contains(resultText(res), "Error") {
		t.Fatalf("append errored: %s", resultText(res))
	}

	head := mustHead(t, reg.Server, posted)
	if !strings.Contains(head, "Original body that the caller must not have to resend.") {
		t.Error("append dropped the original body")
	}
	if !strings.Contains(head, "A correction added later.") {
		t.Error("append did not add the new text")
	}
}

func TestUpdateMessageAppendRejectsContentTogether(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "app-room2")
	posted := mustPost(t, reg.Server, "app-room2", "claude", "body")

	res, _, _ := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: posted, Content: "replace", Append: "add",
	})
	if !strings.Contains(resultText(res), "not both") {
		t.Errorf("expected content+append to be rejected, got: %s", resultText(res))
	}
}

// The read-then-write in append is guarded by expected_content. A concurrent
// edit between the read and the write must fail the append rather than silently
// discard the other edit by appending to a stale body.
func TestUpdateMessageAppendFailsOnConcurrentEdit(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "app-room3")
	posted := mustPost(t, reg.Server, "app-room3", "claude", "v1 body")

	// Simulate the racing edit by passing a stale expected_content explicitly:
	// the caller believed the body was "v0 body" when it is "v1 body".
	res, _, _ := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: posted, Append: "late addition", ExpectedContent: "v0 body",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Errorf("append against a stale expected_content should fail, got: %s", resultText(res))
	}
	if strings.Contains(mustHead(t, reg.Server, posted), "late addition") {
		t.Error("a failed append still modified the message")
	}
}

// mustHead returns the live head content for a message, following one revision.
func mustHead(t *testing.T, s *council.Server, id string) string {
	t.Helper()
	hist, err := s.GetRevisionHistory(id)
	if err != nil || len(hist) == 0 {
		m, err2 := s.GetMessageByID(id)
		if err2 != nil {
			t.Fatalf("read head: %v / %v", err, err2)
		}
		return m.Content
	}
	return hist[len(hist)-1].Content
}
