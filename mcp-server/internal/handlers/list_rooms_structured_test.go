package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestListRoomsStructuredLocal(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "sl-a", withProject("sl-proj"), withTags("x,y"), withRelatedRooms("sl-b"), withDescription("Topic A"))
	mustCreateRoom(t, reg.Server, "sl-b", withProject("sl-proj"))
	mustCreateRoom(t, reg.Server, "sl-done", withProject("sl-proj"))
	if err := reg.Server.UpdateStatus("sl-done", "resolved"); err != nil {
		t.Fatalf("status: %v", err)
	}

	res, out, err := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Project: "sl-proj"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(res), "sl-a") {
		t.Fatalf("markdown lost: %s", resultText(res))
	}
	if len(out.Rooms) != 2 {
		t.Fatalf("want 2 rooms, got %d", len(out.Rooms))
	}
	if out.OmittedResolved != 1 {
		t.Errorf("omitted_resolved = %d, want 1", out.OmittedResolved)
	}
	var a ListedRoom
	for _, rm := range out.Rooms {
		if rm.ID == "sl-a" {
			a = rm
		}
	}
	if a.Topic != "Topic A" || a.Project != "sl-proj" || a.Status != "active" || a.Updated == "" {
		t.Errorf("bad row: %+v", a)
	}
	if len(a.Tags) != 2 || a.Tags[0] != "x" || len(a.RelatedRooms) != 1 || a.RelatedRooms[0] != "sl-b" {
		t.Errorf("tags/related wrong: %+v", a)
	}
	if out.Warnings == nil {
		t.Error("warnings must be a non-nil slice")
	}
}

func TestListRoomsStructuredEmpty(t *testing.T) {
	reg := setupHandlerTest(t)
	_, out, err := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Project: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	if !strings.Contains(string(b), `"rooms":[]`) || !strings.Contains(string(b), `"warnings":[]`) {
		t.Errorf("empty arrays expected, got %s", b)
	}
}

func TestListRoomsStructuredCluster(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "slc-a", withProject("slc-proj"))
	res, out, err := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Project: "slc-proj", ClusterWide: "true"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(res), "slc-a") {
		t.Fatalf("markdown lost: %s", resultText(res))
	}
	if len(out.Rooms) != 1 || out.Rooms[0].ID != "slc-a" || out.Rooms[0].SourceNode == "" {
		t.Errorf("bad cluster rows: %+v", out.Rooms)
	}
	if len(out.Warnings) == 0 {
		t.Error("expected peer fan-out warning in structured warnings")
	}
}

func TestListRoomsStructuredOverMCP(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "slm-a", withProject("slm-proj"))
	for _, args := range []map[string]any{
		{"project": "slm-proj"},
		{"project": "nobody"},
		{"project": "slm-proj", "cluster_wide": "true"},
	} {
		res := callTool(t, cs, "list_rooms", args)
		if res.IsError {
			t.Fatalf("error: %s", resultText(res))
		}
		m, ok := res.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("structuredContent type %T", res.StructuredContent)
		}
		if _, ok := m["rooms"]; !ok {
			t.Errorf("no rooms key: %v", m)
		}
		if _, bad := m["message"]; bad {
			t.Errorf("legacy message key present: %v", m)
		}
	}
}

func TestOnlyStructuredToolsAdvertiseOutputSchema(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		has := tool.OutputSchema != nil
		if tool.Name == "list_rooms" && !has {
			t.Error("list_rooms must advertise an outputSchema")
		}
		if tool.Name != "list_rooms" && has {
			t.Errorf("%s advertises an outputSchema but returns only text", tool.Name)
		}
	}
	res := callTool(t, cs, "load_resources", map[string]any{"uri": "council://guide"})
	if res.StructuredContent != nil {
		t.Errorf("text-only tool returned structuredContent: %v", res.StructuredContent)
	}
}
