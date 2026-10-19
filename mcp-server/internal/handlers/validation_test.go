package handlers

import (
	"strings"
	"testing"
)

func TestUnknownParameterListsAcceptedAndSuggests(t *testing.T) {
	cs, _ := setupIntegrationTest(t)

	res := callTool(t, cs, "room_stats", map[string]any{"room_id": "x", "limit": "5"})
	text := resultText(res)
	if !res.IsError || !strings.Contains(text, "unexpected additional properties") {
		t.Fatalf("expected validation error, got: %s", text)
	}
	if !strings.Contains(text, "accepted parameters for room_stats") || !strings.Contains(text, "room_ids") {
		t.Errorf("error should list accepted parameters: %s", text)
	}

	res = callTool(t, cs, "read_room", map[string]any{"room_id": "x", "include_last": "5"})
	if text = resultText(res); !strings.Contains(text, `did you mean "include_last_n"`) {
		t.Errorf("expected close-name suggestion: %s", text)
	}
}

func TestSuggestParam(t *testing.T) {
	accepted := []string{"room_id", "include_last_n", "cluster_wide", "status"}
	cases := map[string]string{"include_last": "include_last_n", "stauts": "status", "room": "room_id", "zzzz": ""}
	for in, want := range cases {
		if got := suggestParam(in, accepted); got != want {
			t.Errorf("suggestParam(%q)=%q want %q", in, got, want)
		}
	}
}
