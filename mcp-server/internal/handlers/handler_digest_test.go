package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"council-hub/internal/council"
)

func digestRoomsOf(t *testing.T, text string) (map[string]any, []map[string]any) {
	t.Helper()
	var out struct {
		Summary map[string]any   `json:"summary"`
		Rooms   []map[string]any `json:"rooms"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("digest is not JSON: %v\n%s", err, text)
	}
	return out.Summary, out.Rooms
}

// toolHasProp reports whether the registered tool's input schema declares prop.
func toolHasProp(t *testing.T, tool, prop string) bool {
	t.Helper()
	cs, _ := setupIntegrationTest(t)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range tools.Tools {
		if tl.Name != tool {
			continue
		}
		raw, _ := json.Marshal(tl.InputSchema)
		var s struct {
			Properties map[string]any `json:"properties"`
		}
		_ = json.Unmarshal(raw, &s)
		_, ok := s.Properties[prop]
		return ok
	}
	t.Fatalf("tool %s not registered", tool)
	return false
}

func TestGetDigestSchemaAdvertisesLimitAndBrief(t *testing.T) {
	// The handler struct accepts limit; the schema must too, or the "retry with
	// limit=N" advice in the description is a trap.
	for _, name := range []string{"limit", "brief"} {
		if !toolHasProp(t, "get_digest", name) {
			t.Errorf("get_digest schema is missing %q", name)
		}
	}
}

func TestGetDigestLimitReportsOmitted(t *testing.T) {
	reg := setupHandlerTest(t)
	for _, id := range []string{"dl-a", "dl-b", "dl-c"} {
		mustCreateRoom(t, reg.Server, id, withProject("dlproj"))
		mustPost(t, reg.Server, id, "Claude", "hello "+id)
	}
	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{Project: "dlproj", Since: "2000-01-01T00:00:00", Limit: "2"})
	if err != nil {
		t.Fatal(err)
	}
	summary, rooms := digestRoomsOf(t, resultText(res))
	if len(rooms) != 2 {
		t.Fatalf("want 2 rooms, got %d", len(rooms))
	}
	if summary["total"].(float64) != 3 || summary["truncated"].(float64) != 1 {
		t.Errorf("summary must report totals and omitted: %v", summary)
	}
}

func TestGetDigestBriefIsCompact(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "db-a", withProject("dbproj"))
	mustPost(t, reg.Server, "db-a", "Claude", "A long first sentence that would normally become the excerpt.")
	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{Project: "dbproj", Since: "2000-01-01T00:00:00", Brief: "true"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(res)
	if strings.Contains(text, "latest_excerpt") || strings.Contains(text, "\"tags\"") {
		t.Errorf("brief must omit excerpt and tags: %s", text)
	}
	_, rooms := digestRoomsOf(t, text)
	if len(rooms) != 1 || rooms[0]["room_id"] != "db-a" || rooms[0]["latest_message_id"] == "" || rooms[0]["new_messages"].(float64) != 1 {
		t.Errorf("unexpected brief rooms: %v", rooms)
	}
}

func TestGetDigestUnreadOnlyDefaultCapMostUnreadFirst(t *testing.T) {
	reg := setupHandlerTest(t)
	n := digestUnreadDefaultCap + 5
	for i := 0; i < n; i++ {
		id := "du-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		mustCreateRoom(t, reg.Server, id, withProject("duproj"))
		mustPost(t, reg.Server, id, "Claude", "x")
	}
	// A room with extra traffic must survive the cap and lead.
	last := "du-" + string(rune('a'+(n-1)%26)) + string(rune('a'+(n-1)/26))
	mustPost(t, reg.Server, last, "Claude", "y")
	mustPost(t, reg.Server, last, "Claude", "z")
	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{Project: "duproj", UnreadOnly: "true", Agent: "nobody"})
	if err != nil {
		t.Fatal(err)
	}
	summary, rooms := digestRoomsOf(t, resultText(res))
	if len(rooms) != digestUnreadDefaultCap {
		t.Fatalf("want default cap %d, got %d", digestUnreadDefaultCap, len(rooms))
	}
	if rooms[0]["room_id"] != last {
		t.Errorf("most-unread room must come first, got %v", rooms[0]["room_id"])
	}
	if summary["truncated"].(float64) != 5 || summary["total"].(float64) != float64(n) {
		t.Errorf("summary must report omitted: %v", summary)
	}
	res, _, _ = reg.handleGetDigest(context.Background(), nil, DigestInput{Project: "duproj", UnreadOnly: "true", Agent: "nobody", Limit: "all"})
	_, rooms = digestRoomsOf(t, resultText(res))
	if len(rooms) != n {
		t.Errorf("limit=all should return everything, got %d", len(rooms))
	}
}

func TestGetDigestClusterWideDefaultsSince(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{}, "warnings": []string{}})
	}))
	defer srv.Close()
	reg := setupHandlerTest(t)
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	reg.PhoenixURL = srv.URL
	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{ClusterWide: "true"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(resultText(res), "since is required") {
		t.Errorf("cluster_wide without since must default it like the local path: %s", resultText(res))
	}
}

func TestGetDigestLocalNoteWhenClustered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"nodes": []map[string]any{{"node": "a@1"}, {"node": "b@2"}}})
	}))
	defer srv.Close()
	reg := setupHandlerTest(t)
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	reg.PhoenixURL = srv.URL
	mustCreateRoom(t, reg.Server, "dn-a", withProject("dnproj"))
	mustPost(t, reg.Server, "dn-a", "Claude", "x")
	res, _, _ := reg.handleGetDigest(context.Background(), nil, DigestInput{Project: "dnproj", Since: "2000-01-01T00:00:00"})
	var out struct {
		Note string `json:"note"`
	}
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatal(err)
	}
	if out.Note != "local node only; 2 nodes in the cluster" {
		t.Errorf("note = %q", out.Note)
	}
}

func TestGetDigestShowsPostedFromFullNotBrief(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "dpf-h", withProject("dpfproj"))
	for _, p := range []struct{ author, from string }{{"alice", "/w/alpha"}, {"bob", "/w/beta"}} {
		if _, _, err := reg.Server.PostMessageOpts("dpf-h", p.author, "hi", "note", "", "", "", council.PostOptions{PostedFrom: p.from}); err != nil {
			t.Fatal(err)
		}
	}
	in := DigestInput{Project: "dpfproj", Since: "2000-01-01T00:00:00"}
	res, _, _ := reg.handleGetDigest(context.Background(), nil, in)
	_, rooms := digestRoomsOf(t, resultText(res))
	pf, _ := rooms[0]["posted_from"].([]any)
	if len(pf) != 2 || pf[0] != "alpha" || pf[1] != "beta" {
		t.Errorf("full digest posted_from = %v", rooms[0]["posted_from"])
	}
	in.Brief = "true"
	res, _, _ = reg.handleGetDigest(context.Background(), nil, in)
	if strings.Contains(resultText(res), "posted_from") {
		t.Errorf("brief must not carry posted_from: %s", resultText(res))
	}
}
