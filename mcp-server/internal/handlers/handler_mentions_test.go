package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetMentionsSchemaAcceptsClusterWide(t *testing.T) {
	if !toolHasProp(t, "get_mentions", "cluster_wide") {
		t.Error("get_mentions schema is missing cluster_wide")
	}
}

func TestGetMentionsClusterWideMergesPeers(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{
				"id": "019f0000-0000-7000-8000-000000000001", "room_id": "peer-room", "author": "bob",
				"content": "ping from peer", "timestamp": "2026-10-01T10:00:00Z", "source_node": "peer@10.0.0.2",
			}},
			"warnings": []string{},
		})
	}))
	defer srv.Close()
	reg := setupHandlerTest(t)
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	reg.PhoenixURL = srv.URL
	mustCreateRoom(t, reg.Server, "gm-local")
	_, _ = reg.Server.PostMessageWithMentions("gm-local", "alice", "local ping", "message", "", "claude")

	res, _, err := reg.handleGetMentions(context.Background(), nil, GetMentionsInput{Author: "claude", ClusterWide: "true"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(res)
	if gotPath != "/api/internal/cluster/get_mentions" {
		t.Errorf("fan-out path = %q", gotPath)
	}
	for _, want := range []string{"local ping", "ping from peer", "peer@10.0.0.2"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in: %s", want, text)
		}
	}
}

func TestGetMentionsLocalNoteWhenClustered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"nodes": []map[string]any{{"node": "a@1"}, {"node": "b@2"}, {"node": "c@3"}}})
	}))
	defer srv.Close()
	reg := setupHandlerTest(t)
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	reg.PhoenixURL = srv.URL
	res, _, _ := reg.handleGetMentions(context.Background(), nil, GetMentionsInput{Author: "claude", ClusterWide: "false"})
	text := strings.TrimSpace(resultText(res))
	if !strings.HasSuffix(text, "local node only; 3 nodes in the cluster") {
		t.Errorf("want footer, got: %s", text)
	}
}

func TestGetMentionsNoFooterOnSingleNode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"nodes": []map[string]any{{"node": "a@1"}}})
	}))
	defer srv.Close()
	reg := setupHandlerTest(t)
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	reg.PhoenixURL = srv.URL
	res, _, _ := reg.handleGetMentions(context.Background(), nil, GetMentionsInput{Author: "claude"})
	if strings.Contains(resultText(res), "local node only") {
		t.Errorf("single node must not get a footer: %s", resultText(res))
	}
}
