package handlers

import (
	"context"
	"strings"
	"testing"
)

func TestHandleLinkAndGetLinks(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-links")
	a := mustPost(t, reg.Server, "h-links", "Claude", "decision A")
	b := mustPost(t, reg.Server, "h-links", "Gemini", "B refines A")

	res, _, _ := reg.handleLinkMessages(context.Background(), nil, LinkMessagesInput{
		FromID: b, ToID: a, Relation: "refines", Author: "Gemini",
	})
	if !strings.Contains(resultText(res), "Linked") {
		t.Errorf("expected link confirmation, got: %s", resultText(res))
	}

	// get_links on A shows the incoming backlink.
	res2, _, _ := reg.handleGetLinks(context.Background(), nil, GetLinksInput{MessageID: a})
	text := resultText(res2)
	if !strings.Contains(text, "Incoming") || !strings.Contains(text, "refines") {
		t.Errorf("expected incoming refines backlink, got: %s", text)
	}
}

func TestHandleLinkInvalidRelation(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-links-bad")
	a := mustPost(t, reg.Server, "h-links-bad", "Claude", "a")
	b := mustPost(t, reg.Server, "h-links-bad", "Claude", "b")

	res, _, _ := reg.handleLinkMessages(context.Background(), nil, LinkMessagesInput{
		FromID: a, ToID: b, Relation: "bogus",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") || !strings.Contains(text, "refines") {
		t.Errorf("expected error listing valid relations, got: %s", text)
	}
}

func TestHandleUnlinkMessages(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-unlink")
	a := mustPost(t, reg.Server, "h-unlink", "Claude", "a")
	b := mustPost(t, reg.Server, "h-unlink", "Claude", "b")
	id, _ := reg.Server.CreateLink(a, b, "relates", "")

	res, _, _ := reg.handleUnlinkMessages(context.Background(), nil, UnlinkMessagesInput{LinkID: id})
	if !strings.Contains(resultText(res), "removed") {
		t.Errorf("expected removal confirmation, got: %s", resultText(res))
	}

	out, _, _ := reg.Server.GetLinks(a)
	if len(out) != 0 {
		t.Errorf("expected no links after unlink, got %d", len(out))
	}
}

func TestHandleGetLinksNoLinks(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-nolinks")
	a := mustPost(t, reg.Server, "h-nolinks", "Claude", "lonely")

	res, _, _ := reg.handleGetLinks(context.Background(), nil, GetLinksInput{MessageID: a})
	if !strings.Contains(resultText(res), "no links") {
		t.Errorf("expected 'no links' message, got: %s", resultText(res))
	}
}

func TestParseDepth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 1},
		{"3", 3},
		{"0", 0},
		{"-2", -2},
		{"abc", 1},
		{"2x", 2},
	}
	for _, c := range cases {
		if got := parseDepth(c.in); got != c.want {
			t.Errorf("parseDepth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestImplicitTag(t *testing.T) {
	if got := implicitTag(true, "ignored"); got != " _(implicit)_" {
		t.Errorf("implicit tag = %q", got)
	}
	if got := implicitTag(false, "abc"); got != " _(link #abc)_" {
		t.Errorf("explicit tag = %q", got)
	}
}

func TestRenderNeighborhood(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "nb")
	a := mustPost(t, reg.Server, "nb", "Claude", "root claim")
	b, _ := reg.Server.PostMessage("nb", "Gemini", "refinement", "decision", "")
	c := mustPost(t, reg.Server, "nb", "Codex", "follow-up")
	l1, err := reg.Server.CreateLink(b, a, "refines", "Gemini")
	if err != nil {
		t.Fatal(err)
	}
	l2, err := reg.Server.CreateLink(c, b, "relates", "Codex")
	if err != nil {
		t.Fatal(err)
	}

	// depth 1 reaches b only
	res, _, _ := reg.renderNeighborhood(a, 1)
	text := resultText(res)
	if !strings.Contains(text, "Link neighborhood of #"+a+" (depth 1): 2 node(s), 1 edge(s)") {
		t.Errorf("depth 1 header wrong:\n%s", text)
	}
	if strings.Contains(text, "#"+c) {
		t.Errorf("depth 1 must not reach c:\n%s", text)
	}

	// depth 2 reaches c, grouped by distance, with typed tags and edges
	res, _, _ = reg.renderNeighborhood(a, 2)
	text = resultText(res)
	for _, want := range []string{
		"(depth 2): 3 node(s), 2 edge(s)",
		"**Distance 0:**\n  - #" + a + " [Claude] root claim",
		"**Distance 1:**\n  - #" + b + " [Gemini decision] refinement",
		"**Distance 2:**\n  - #" + c + " [Codex] follow-up",
		"**Edges:**",
		"- #" + b + " --refines--> #" + a + " _(link #" + l1 + ")_",
		"- #" + c + " --relates--> #" + b + " _(link #" + l2 + ")_",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Index(text, "Distance 0") > strings.Index(text, "Distance 1") || strings.Index(text, "Distance 1") > strings.Index(text, "Distance 2") {
		t.Errorf("distances out of order:\n%s", text)
	}

	// same output reached through get_links depth=2
	res, _, _ = reg.handleGetLinks(context.Background(), nil, GetLinksInput{MessageID: a, Depth: "2"})
	if resultText(res) != text {
		t.Errorf("get_links depth=2 differs from renderNeighborhood")
	}

	// unknown message
	res, _, _ = reg.renderNeighborhood("00000000-0000-7000-8000-000000000000", 2)
	if !strings.HasPrefix(resultText(res), "Error: message 00000000 not found") {
		t.Errorf("expected not-found error, got: %s", resultText(res))
	}
}
