package handlers

import (
	"strings"
	"testing"
)

func ackRow(t *testing.T, reg *Registry, agent, id string) (string, bool) {
	t.Helper()
	var at string
	err := reg.Server.DB.QueryRow(`SELECT acked_at FROM mention_acks WHERE agent = ? AND message_id = ?`, agent, id).Scan(&at)
	return at, err == nil
}

func TestMentionAckRecordsReceiptsAndKeepsFirstTime(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ack-room")
	id1 := postPing(t, reg, "ack-room", "first ping", "claude")
	id2 := postPing(t, reg, "ack-room", "second ping", "claude")

	ackMentions(t, reg, GetMentionsInput{Author: "Claude", Ack: "true"})
	if _, ok := ackRow(t, reg, "claude", id1); !ok { // key is lowercased
		t.Fatal("receipt for first mention missing")
	}
	if _, ok := ackRow(t, reg, "claude", id2); !ok {
		t.Fatal("receipt for second mention missing")
	}

	// Backdate, then re-ack the same ids: the first time must survive.
	if _, err := reg.Server.DB.Exec(`UPDATE mention_acks SET acked_at = '2020-01-01 00:00:00'`); err != nil {
		t.Fatal(err)
	}
	if err := reg.Server.AckMentions("  CLAUDE ", []string{id1, id2}); err != nil { // trimmed too
		t.Fatal(err)
	}
	if at, _ := ackRow(t, reg, "claude", id1); !strings.HasPrefix(at, "2020-01-01") { // driver may reformat the stamp
		t.Errorf("re-ack changed acked_at to %q", at)
	}
	var n int
	_ = reg.Server.DB.QueryRow(`SELECT COUNT(*) FROM mention_acks`).Scan(&n)
	if n != 2 {
		t.Errorf("rows = %d, want 2", n)
	}
}

func TestMentionNoAckRecordsNothing(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ack-room")
	postPing(t, reg, "ack-room", "first ping", "claude")
	ackMentions(t, reg, GetMentionsInput{Author: "claude"})
	var n int
	_ = reg.Server.DB.QueryRow(`SELECT COUNT(*) FROM mention_acks`).Scan(&n)
	if n != 0 {
		t.Errorf("ack=false recorded %d rows", n)
	}
}
