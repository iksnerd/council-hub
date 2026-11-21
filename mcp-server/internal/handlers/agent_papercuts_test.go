package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Every array-typed property must say it takes a JSON array or ONE
// comma-separated string, so no agent has to guess the spelling.
func TestEveryArrayPropertyStatesAcceptedForms(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	const want = "JSON array of strings or ONE comma-separated string"
	seen := 0
	for _, tool := range list.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		var s struct {
			Properties map[string]struct {
				Type        string `json:"type"`
				Description string `json:"description"`
			} `json:"properties"`
		}
		_ = json.Unmarshal(raw, &s)
		for name, p := range s.Properties {
			if p.Type != "array" {
				continue
			}
			seen++
			if !strings.Contains(p.Description, want) {
				t.Errorf("%s.%s description lacks %q: %s", tool.Name, name, want, p.Description)
			}
		}
	}
	if seen == 0 {
		t.Fatal("found no array properties")
	}
}

func TestMalformedArrayErrorNamesFieldAndExample(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	res := callTool(t, cs, "create_room", map[string]any{"id": "x", "tags": 5})
	text := resultText(res)
	if !res.IsError {
		t.Fatalf("expected an error, got %s", text)
	}
	if !strings.Contains(text, `"tags" takes a JSON array of strings or ONE comma-separated string`) ||
		!strings.Contains(text, `tags="a,b"`) {
		t.Errorf("error should name the field and give an example: %s", text)
	}
}

func TestClampedLimitIsReported(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	res := callTool(t, cs, "search_messages", map[string]any{"query": "x", "limit": 500})
	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(res))
	}
	last := res.Content[len(res.Content)-1].(interface{ MarshalJSON() ([]byte, error) })
	b, _ := last.MarshalJSON()
	if !strings.Contains(string(b), "limit clamped to 100; you sent 500") {
		t.Errorf("missing clamp note: %s", b)
	}
	if strings.Contains(resultText(res), "clamped") {
		t.Errorf("original reply text must be unchanged: %s", resultText(res))
	}

	// In range: nothing appended.
	res = callTool(t, cs, "search_messages", map[string]any{"query": "x", "limit": 5})
	if len(res.Content) != 1 {
		t.Errorf("in-range limit must add no note, got %d content items", len(res.Content))
	}
}
