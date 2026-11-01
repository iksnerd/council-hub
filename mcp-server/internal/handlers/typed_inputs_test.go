package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// drenskicorp RFC 0013 section 4: a property is a string, an integer, a boolean
// or an array of strings. The hub used to advertise every property as a string;
// the schema now says what each one is, and the server still takes the old string
// spellings so no client breaks.

func toolPropTypes(t *testing.T) map[string]map[string]string {
	t.Helper()
	cs, _ := setupIntegrationTest(t)
	return listPropTypes(t, cs)
}

// setupWithEmbedder is setupIntegrationTest with an embedder configured before
// the tools register, so the embedder-only properties (search_messages.semantic)
// are in the schema.
func setupWithEmbedder(t *testing.T) *mcp.ClientSession {
	t.Helper()
	s := setupTestServer(t)
	s.Embedder = &stubEmbedder{vec: make([]float32, 768)}
	reg := &Registry{Server: s}
	reg.RegisterTools()
	reg.RegisterResources()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := s.MCP.Connect(context.Background(), t1, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil).Connect(context.Background(), t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func listPropTypes(t *testing.T, cs *mcp.ClientSession) map[string]map[string]string {
	t.Helper()
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, tool := range list.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		var s struct {
			Properties map[string]struct {
				Type  string `json:"type"`
				Items struct {
					Type string `json:"type"`
				} `json:"items"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		out[tool.Name] = map[string]string{}
		for k, v := range s.Properties {
			ty := v.Type
			if ty == "array" {
				ty += "<" + v.Items.Type + ">"
			}
			out[tool.Name][k] = ty
		}
	}
	return out
}

func TestSchemaAdvertisesRealPropertyTypes(t *testing.T) {
	cs := setupWithEmbedder(t)
	types := listPropTypes(t, cs)
	want := map[string]string{
		"get_digest.brief":              "boolean",
		"search_messages.summary_only":  "boolean",
		"post_to_room.pin":              "boolean",
		"search_messages.limit":         "integer",
		"get_links.depth":               "integer",
		"search_messages.excerpt_chars": "integer",
		"get_messages.message_ids":      "array<string>",
		"post_to_room.mentions":         "array<string>",
		"update_room.add_tags":          "array<string>",
		"get_digest.limit":              "string", // also accepts "all"
		"search_messages.query":         "string",
		"search_messages.semantic":      "boolean", // only present with an embedder
	}
	for key, ty := range want {
		parts := strings.SplitN(key, ".", 2)
		if got := types[parts[0]][parts[1]]; got != ty {
			t.Errorf("%s: type = %q, want %q", key, got, ty)
		}
	}
}

func TestEveryPropertyIsOneOfTheFourRFCShapes(t *testing.T) {
	for name, types := range map[string]map[string]map[string]string{
		"no embedder":   toolPropTypes(t),
		"with embedder": listPropTypes(t, setupWithEmbedder(t)),
	} {
		for tool, props := range types {
			for prop, ty := range props {
				switch ty {
				case "string", "integer", "boolean", "array<string>":
				default:
					t.Errorf("[%s] %s.%s has type %q, outside RFC 0013 section 4", name, tool, prop, ty)
				}
			}
		}
	}
}

// RFC 0013 section 4 allows only these keywords on a property, by type.
func TestPropertyKeywordsStayWithinTheRFC(t *testing.T) {
	allowed := map[string]map[string]bool{
		"string":  {"type": true, "description": true, "default": true, "enum": true},
		"boolean": {"type": true, "description": true, "default": true},
		"integer": {"type": true, "description": true, "default": true, "minimum": true, "maximum": true},
		"array":   {"type": true, "description": true, "items": true},
	}
	for name, cs := range map[string]*mcp.ClientSession{"plain": func() *mcp.ClientSession { c, _ := setupIntegrationTest(t); return c }(), "embedder": setupWithEmbedder(t)} {
		list, err := cs.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, tool := range list.Tools {
			raw, _ := json.Marshal(tool.InputSchema)
			var s struct {
				Properties map[string]map[string]json.RawMessage `json:"properties"`
			}
			_ = json.Unmarshal(raw, &s)
			for p, kws := range s.Properties {
				var ty string
				_ = json.Unmarshal(kws["type"], &ty)
				for kw := range kws {
					if !allowed[ty][kw] {
						t.Errorf("[%s] %s.%s (%s) uses keyword %q", name, tool.Name, p, ty, kw)
					}
				}
			}
		}
	}
}

func TestIntegerRangesAreAdvertised(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{ // -1 = no bound
		"search_messages.limit":         {-1, 100},
		"search_messages.excerpt_chars": {40, 2000},
		"get_messages.last_n":           {-1, 50},
		"get_messages.limit":            {-1, 200},
		"get_links.depth":               {-1, 5},
		"list_rooms.offset":             {0, -1},
		"read_notebook.level":           {0, -1},
	}
	got := map[string][2]int{}
	for _, tool := range list.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		var s struct {
			Properties map[string]struct {
				Minimum *int `json:"minimum"`
				Maximum *int `json:"maximum"`
			} `json:"properties"`
		}
		_ = json.Unmarshal(raw, &s)
		for p, v := range s.Properties {
			b := [2]int{-1, -1}
			if v.Minimum != nil {
				b[0] = *v.Minimum
			}
			if v.Maximum != nil {
				b[1] = *v.Maximum
			}
			got[tool.Name+"."+p] = b
		}
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: bounds = %v, want %v", k, got[k], w)
		}
	}
}

func i64(n int64) *int64 { return &n }

func TestCoerceClampsIntegersIntoRange(t *testing.T) {
	spec := map[string]propSpec{"limit": {Type: "integer", Min: i64(0), Max: i64(100)}}
	cases := map[string]string{
		`{"limit":50}`:     `{"limit":50}`,     // in range: untouched
		`{"limit":500}`:    `{"limit":100}`,    // over max: clamped
		`{"limit":"500"}`:  `{"limit":100}`,    // legacy string, clamped
		`{"limit":-5}`:     `{"limit":0}`,      // below min: clamped
		`{"limit":"lots"}`: `{"limit":"lots"}`, // left for the validator
	}
	for in, want := range cases {
		if got := string(coerceArguments(json.RawMessage(in), spec)); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
}

func TestNativeNumberToStringPropertyWorks(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	for _, v := range []any{2, "2", "all"} {
		res := callTool(t, cs, "get_digest", map[string]any{"limit": v})
		if res.IsError {
			t.Errorf("get_digest(limit=%v): %s", v, resultText(res))
		}
	}
}

func TestOverMaxIsClampedNotRejected(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	res := callTool(t, cs, "search_messages", map[string]any{"query": "x", "limit": 100000})
	if res.IsError {
		t.Errorf("over-max limit should be clamped: %s", resultText(res))
	}
	res = callTool(t, cs, "search_messages", map[string]any{"query": "x", "limit": "lots"})
	if !res.IsError || !strings.Contains(resultText(res), "limit") {
		t.Errorf("limit=lots should be refused by name: %s", resultText(res))
	}
}

func TestTypedAndLegacyStringInputsBothWork(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	callTool(t, cs, "create_room", map[string]any{"id": "typed-in", "topic": "x"})
	for i := 0; i < 3; i++ {
		callTool(t, cs, "post_to_room", map[string]any{"room_id": "typed-in", "author": "a", "message": "msg number " + strings.Repeat("x", 200)})
	}
	cases := map[string]map[string]any{
		"typed":  {"room_id": "typed-in", "limit": 2, "summary_only": true, "excerpt_chars": 60},
		"legacy": {"room_id": "typed-in", "limit": "2", "summary_only": "true", "excerpt_chars": "60"},
	}
	for name, args := range cases {
		res := callTool(t, cs, "search_messages", args)
		if res.IsError {
			t.Fatalf("%s: %s", name, resultText(res))
		}
		if txt := resultText(res); !strings.Contains(txt, "2 message(s)") || !strings.Contains(txt, "|") {
			t.Errorf("%s: limit/summary_only not honoured: %s", name, txt)
		}
	}
}

func TestListInputsAcceptArrayOrCommaString(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	callTool(t, cs, "create_room", map[string]any{"id": "arr-in", "topic": "x"})
	var ids []string
	for i := 0; i < 2; i++ {
		callTool(t, cs, "post_to_room", map[string]any{"room_id": "arr-in", "author": "a", "message": "m"})
	}
	msgs, _ := reg.Server.GetRecentMessages("arr-in", 5)
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}
	for name, v := range map[string]any{"array": ids, "comma": strings.Join(ids, ","), "json-string": mustJSON(ids)} {
		res := callTool(t, cs, "get_messages", map[string]any{"message_ids": v})
		if res.IsError || strings.Contains(resultText(res), "Not found") {
			t.Errorf("%s: %s", name, resultText(res))
		}
	}
}

func TestWrongTypeIsStillRejected(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	res := callTool(t, cs, "search_messages", map[string]any{"limit": "lots"})
	if !res.IsError {
		t.Errorf("a non-numeric limit should be a tool error: %s", resultText(res))
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
