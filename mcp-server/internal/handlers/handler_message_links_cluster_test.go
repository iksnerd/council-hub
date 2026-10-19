package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLinkMessagesMissingIDSaysLinksAreNodeLocal(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "lk-local")
	a := mustPost(t, reg.Server, "lk-local", "alice", "local message")

	res, _, _ := reg.handleLinkMessages(context.Background(), nil, LinkMessagesInput{
		FromID: a, ToID: "00000000-0000-7000-8000-000000000000", Relation: "relates",
	})
	text := resultText(res)
	if !strings.Contains(text, "node-local") {
		t.Errorf("expected node-local explanation, got: %s", text)
	}
	if !strings.Contains(text, "00000000") {
		t.Errorf("expected the missing id to be named, got: %s", text)
	}
}

func TestLinkMessagesNamesOwningNodeOfMissingID(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "lk-here")
	a := mustPost(t, reg.Server, "lk-here", "alice", "local message")
	remoteID := "11111111-0000-7000-8000-000000000000"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/cluster/get_messages") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results":  []map[string]any{{"id": remoteID, "room_id": "peer-room", "source_node": "peer@10.0.0.9"}},
				"warnings": []string{},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	reg.PhoenixURL = server.URL
	reg.HTTPClient = server.Client()

	res, _, _ := reg.handleLinkMessages(context.Background(), nil, LinkMessagesInput{
		FromID: a, ToID: remoteID, Relation: "relates",
	})
	text := resultText(res)
	if !strings.Contains(text, "peer@10.0.0.9") || !strings.Contains(text, "peer-room") {
		t.Errorf("expected owning node and room named, got: %s", text)
	}
	if !strings.Contains(text, "node-local") {
		t.Errorf("expected node-local explanation, got: %s", text)
	}
}
