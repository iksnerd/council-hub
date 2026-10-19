package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func uiPost(t *testing.T, reg *Registry, method, remote, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/ui/post", strings.NewReader(body))
	req.RemoteAddr = remote
	w := httptest.NewRecorder()
	reg.UIPostHandler()(w, req)
	return w
}

func TestUIPostHandlerRejectsNonLocalhost(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ui-post")
	w := uiPost(t, reg, http.MethodPost, "10.0.0.9:54321", `{"room_id":"ui-post","author":"a","message":"hi"}`)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
	msgs, _ := reg.Server.GetRecentMessages("ui-post", 10)
	if len(msgs) != 0 {
		t.Errorf("a rejected request must not post, got %d message(s)", len(msgs))
	}
}

func TestUIPostHandlerRejectsGET(t *testing.T) {
	reg := setupHandlerTest(t)
	w := uiPost(t, reg, http.MethodGet, "127.0.0.1:54321", "")
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestUIPostHandlerBadBody(t *testing.T) {
	reg := setupHandlerTest(t)
	w := uiPost(t, reg, http.MethodPost, "127.0.0.1:54321", `{not json`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed JSON, got %d", w.Code)
	}
}

// Missing fields are reported in the JSON body with HTTP 200, the same contract
// as UINotebookEntryHandler: the Phoenix client reads the "error" field.
func TestUIPostHandlerMissingFields(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ui-post")
	for _, body := range []string{
		`{"author":"a","message":"hi"}`,
		`{"room_id":"ui-post","message":"hi"}`,
		`{"room_id":"ui-post","author":"a"}`,
	} {
		w := uiPost(t, reg, http.MethodPost, "127.0.0.1:54321", body)
		var resp internalPostResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode %q: %v", w.Body.String(), err)
		}
		if w.Code != http.StatusOK || !strings.Contains(resp.Error, "required") || resp.MessageID != "" {
			t.Errorf("body %s: want 200 + required error, got %d %+v", body, w.Code, resp)
		}
	}
	msgs, _ := reg.Server.GetRecentMessages("ui-post", 10)
	if len(msgs) != 0 {
		t.Errorf("invalid requests must not post, got %d message(s)", len(msgs))
	}
}

func TestUIPostHandlerPostsMessage(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ui-post")

	w := uiPost(t, reg, http.MethodPost, "127.0.0.1:54321",
		`{"room_id":"ui-post","author":"human","message":"from the composer","message_type":"note"}`)
	var resp internalPostResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if w.Code != http.StatusOK || resp.Error != "" || resp.MessageID == "" || resp.RoomID != "ui-post" {
		t.Fatalf("expected success with message_id, got %d %+v", w.Code, resp)
	}
	m, err := reg.Server.GetMessageByID(resp.MessageID)
	if err != nil {
		t.Fatalf("posted message not stored: %v", err)
	}
	if m.Author != "human" || m.Content != "from the composer" || m.MessageType != "note" || m.RoomID != "ui-post" {
		t.Errorf("stored message wrong: %+v", m)
	}

	// message_type defaults to "message"
	w = uiPost(t, reg, http.MethodPost, "[::1]:54321", `{"room_id":"ui-post","author":"human","message":"plain"}`)
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	m, err = reg.Server.GetMessageByID(resp.MessageID)
	if err != nil || m.MessageType != "message" {
		t.Errorf("default type wrong: %+v err=%v", m, err)
	}
}
