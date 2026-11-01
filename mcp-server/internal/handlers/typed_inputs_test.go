package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// drenskicorp RFC 0013 section 4: a property is a string, an integer, a boolean
// or an array of strings. The hub used to advertise every property as a string;
// the schema now says what each one is, and the server still takes the old string
// spellings so no client breaks.

func toolPropTypes(t *testing.T) map[string]map[string]string {
	t.Helper()
	cs, _ := setupIntegrationTest(t)
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
	types := toolPropTypes(t)
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
	}
	for key, ty := range want {
		parts := strings.SplitN(key, ".", 2)
		if got := types[parts[0]][parts[1]]; got != ty {
			t.Errorf("%s: type = %q, want %q", key, got, ty)
		}
	}
}

func TestEveryPropertyIsOneOfTheFourRFCShapes(t *testing.T) {
	for tool, props := range toolPropTypes(t) {
		for name, ty := range props {
			switch ty {
			case "string", "integer", "boolean", "array<string>":
			default:
				t.Errorf("%s.%s has type %q, outside RFC 0013 section 4", tool, name, ty)
			}
		}
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
