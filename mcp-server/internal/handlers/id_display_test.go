package handlers

import (
	"context"
	"regexp"
	"testing"
)

// Two messages whose ids share the first 8 chars must print distinct prefixes in
// read_transcript and in read_room's recent messages, and each printed prefix
// must resolve back to its own message.
func TestPrintedIDsAreUniqueWithinRoomAndResolve(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "idd-room")
	ids := []string{"01a14ecc-0000-7000-8000-000000000001", "01a14ecc-0000-7000-8000-000000000002"}
	for i, id := range ids {
		if _, err := reg.Server.DB.Exec(`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, 'idd-room', 'a', ?, 'note')`, id, "body-"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	re := regexp.MustCompile(`\[#([0-9a-f-]+) `)

	check := func(name, text string) {
		got := re.FindAllStringSubmatch(text, -1)
		if len(got) != 2 {
			t.Fatalf("%s: want 2 ids, got %v\n%s", name, got, text)
		}
		if got[0][1] == got[1][1] {
			t.Errorf("%s: both messages print %q", name, got[0][1])
		}
		for i, g := range got {
			full, err := reg.Server.ResolveMessageIDInRoom(g[1], "idd-room")
			if err != nil || full != ids[i] {
				t.Errorf("%s: printed #%s resolved to %q, %v; want %q", name, g[1], full, err, ids[i])
			}
		}
	}

	res, _, _ := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{RoomID: "idd-room"})
	check("transcript", resultText(res))

	res, _, _ = reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "idd-room", IncludeLastN: "5"})
	check("read_room", resultText(res))
}
