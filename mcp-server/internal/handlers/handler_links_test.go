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
