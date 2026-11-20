package handlers

import (
	"context"
	"strings"
	"testing"
)

func ackMentions(t *testing.T, reg *Registry, in GetMentionsInput) string {
	t.Helper()
	in.ClusterWide = "false"
	res, _, err := reg.handleGetMentions(context.Background(), nil, in)
	if err != nil {
		t.Fatal(err)
	}
	return resultText(res)
}

func postPing(t *testing.T, reg *Registry, room, body, mentions string) string {
	t.Helper()
	id, err := reg.Server.PostMessageWithMentions(room, "alice", body, "message", "", mentions)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestGetMentionsSchemaAcceptsAck(t *testing.T) {
	if !toolHasProp(t, "get_mentions", "ack") {
		t.Error("get_mentions schema is missing ack")
	}
}

func TestGetMentionsWithoutAckIsUnchanged(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ack-room")
	postPing(t, reg, "ack-room", "first ping", "claude")
	for i := 0; i < 2; i++ {
		text := ackMentions(t, reg, GetMentionsInput{Author: "claude"})
		if !strings.Contains(text, "first ping") || strings.Contains(text, "cursor") {
			t.Fatalf("call %d: %s", i, text)
		}
	}
}

func TestGetMentionsAckAdvancesAndHides(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ack-room")
	id1 := postPing(t, reg, "ack-room", "first ping", "claude")
	id2 := postPing(t, reg, "ack-room", "second ping", "claude")

	text := ackMentions(t, reg, GetMentionsInput{Author: "claude", Ack: "true"})
	for _, want := range []string{"first ping", "second ping", "advanced to #" + id2} {
		if !strings.Contains(text, want) {
			t.Errorf("first ack missing %q: %s", want, text)
		}
	}

	text = ackMentions(t, reg, GetMentionsInput{Author: "claude", Ack: "true"})
	if strings.Contains(text, "first ping") || strings.Contains(text, "advanced") {
		t.Errorf("second ack should return nothing new: %s", text)
	}
	if !strings.Contains(text, "No new mentions") || !strings.Contains(text, "2 older mention(s) hidden") {
		t.Errorf("second ack should report hidden count: %s", text)
	}

	id3 := postPing(t, reg, "ack-room", "third ping", "claude")
	text = ackMentions(t, reg, GetMentionsInput{Author: "claude", Ack: "true"})
	if !strings.Contains(text, "third ping") || strings.Contains(text, "second ping") ||
		!strings.Contains(text, "advanced to #"+id3) || !strings.Contains(text, "2 older") {
		t.Errorf("new mention after ack: %s", text)
	}

	// ack=false still sees everything and does not touch the cursor.
	text = ackMentions(t, reg, GetMentionsInput{Author: "claude"})
	if !strings.Contains(text, "first ping") || !strings.Contains(text, id1) {
		t.Errorf("plain call lost old mentions: %s", text)
	}
}

func TestGetMentionsAckCursorPerAgent(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ack-room")
	postPing(t, reg, "ack-room", "for both", "alpha-agent,beta-agent")
	ackMentions(t, reg, GetMentionsInput{Author: "alpha-agent", Ack: "true"})

	if text := ackMentions(t, reg, GetMentionsInput{Author: "alpha-agent", Ack: "true"}); strings.Contains(text, "for both") {
		t.Errorf("alpha should be caught up: %s", text)
	}
	if text := ackMentions(t, reg, GetMentionsInput{Author: "Beta-Agent", Ack: "true"}); !strings.Contains(text, "for both") {
		t.Errorf("beta has its own cursor: %s", text)
	}
	// Same agent, different case and padding, shares one cursor.
	if text := ackMentions(t, reg, GetMentionsInput{Author: "ALPHA-AGENT", Ack: "true"}); strings.Contains(text, "for both") {
		t.Errorf("case variant should share the cursor: %s", text)
	}
}

func TestGetMentionsAckLimitDoesNotSkip(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ack-room")
	postPing(t, reg, "ack-room", "ping one", "claude")
	postPing(t, reg, "ack-room", "ping two", "claude")
	postPing(t, reg, "ack-room", "ping three", "claude")

	text := ackMentions(t, reg, GetMentionsInput{Author: "claude", Ack: "true", Limit: "2"})
	if !strings.Contains(text, "ping one") || !strings.Contains(text, "ping two") || strings.Contains(text, "ping three") {
		t.Errorf("limit page 1 should be the two oldest unseen: %s", text)
	}
	if !strings.Contains(text, "1 more new mention(s)") {
		t.Errorf("should say one more remains: %s", text)
	}
	text = ackMentions(t, reg, GetMentionsInput{Author: "claude", Ack: "true", Limit: "2"})
	if !strings.Contains(text, "ping three") || strings.Contains(text, "ping two") {
		t.Errorf("limit page 2 should be the remainder: %s", text)
	}
}

func TestGetMentionsAckProjectScope(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ack-room")
	postPing(t, reg, "ack-room", "scoped ping", "claude")
	text := ackMentions(t, reg, GetMentionsInput{Author: "claude", Ack: "true", Project: "no-such-project"})
	if strings.Contains(text, "scoped ping") {
		t.Errorf("project filter ignored with ack: %s", text)
	}
}
