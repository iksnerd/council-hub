package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleRoomStatsCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": map[string]any{
				"room_id":           "stats-room",
				"status":            "active",
				"message_count":     42,
				"participants":      map[string]int{"Claude": 30, "Gemini": 12},
				"type_counts":       map[string]int{"message": 35, "decision": 7},
				"first_message":     "2026-03-01T10:00:00",
				"last_message":      "2026-04-01T12:00:00",
				"latest_message_id": "abc12345",
				"source_node":       "council_hub@10.0.0.5",
			},
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleRoomStatsCluster(RoomStatsInput{RoomID: "stats-room"})
	if err != nil {
		t.Fatalf("handleRoomStatsCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "council_hub@10.0.0.5") {
		t.Errorf("expected node tag, got: %s", text)
	}
	if !strings.Contains(text, "42") {
		t.Errorf("expected message count, got: %s", text)
	}
}

func TestHandleRoomStatsClusterNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results":  nil,
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleRoomStatsCluster(RoomStatsInput{RoomID: "nonexistent"})
	if err != nil {
		t.Fatalf("handleRoomStatsCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "not found on any cluster node") {
		t.Errorf("expected not found message, got: %s", text)
	}
}

func TestHandleRoomStatsBranching(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "branch-stats-room")
	mustPost(t, reg.Server, "branch-stats-room", "Claude", "Hello")

	// Local path (default)
	args := RoomStatsInput{RoomID: "branch-stats-room"}
	result, _, err := reg.handleRoomStats(nil, nil, args)
	if err != nil {
		t.Fatalf("local room stats failed: %v", err)
	}
	text := resultText(result)
	if !strings.Contains(text, "branch-stats-room") {
		t.Errorf("expected local result, got: %s", text)
	}

	// Cluster path (no Phoenix running, should get error message)
	args.ClusterWide = "true"
	result, _, err = reg.handleRoomStats(nil, nil, args)
	if err != nil {
		t.Fatalf("cluster room stats should not return error: %v", err)
	}
	text = resultText(result)
	if !strings.Contains(text, "Error: cluster room stats failed") {
		t.Errorf("expected cluster error, got: %s", text)
	}
}

func TestHandleRoomStatsClusterFullFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": map[string]any{
				"room_id":           "full-stats",
				"status":            "resolved",
				"message_count":     100,
				"participants":      map[string]int{"Alice": 60, "Bob": 40},
				"type_counts":       map[string]int{"message": 70, "decision": 20, "action": 10},
				"first_message":     "2026-01-01T10:00:00",
				"last_message":      "2026-04-01T15:30:00",
				"latest_message_id": "deadbeef",
				"source_node":       "council_hub@10.0.0.5",
			},
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleRoomStatsCluster(RoomStatsInput{RoomID: "full-stats"})
	if err != nil {
		t.Fatalf("full stats failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "resolved") {
		t.Errorf("expected status, got: %s", text)
	}
	if !strings.Contains(text, "100") {
		t.Errorf("expected message count, got: %s", text)
	}
	if !strings.Contains(text, "deadbeef") {
		t.Errorf("expected latest message id, got: %s", text)
	}
	if !strings.Contains(text, "Alice") || !strings.Contains(text, "Bob") {
		t.Errorf("expected participants, got: %s", text)
	}
	if !strings.Contains(text, "decision") {
		t.Errorf("expected type counts, got: %s", text)
	}
	if !strings.Contains(text, "2026-01-01") {
		t.Errorf("expected first message timestamp, got: %s", text)
	}
}
