package council

import (
	"testing"
	"time"
)

func TestNormalizeTimestamp(t *testing.T) {
	cases := []struct {
		in       string
		endOfDay bool
		want     string
	}{
		{"2026-10-18T00:00:00", false, "2026-10-18 00:00:00"},
		{"2026-10-18 00:00:00", false, "2026-10-18 00:00:00"},
		{"2026-10-18T00:00:00Z", false, "2026-10-18 00:00:00"},
		{"2026-10-18T02:00:00+02:00", false, "2026-10-18 00:00:00"},
		{"2026-10-18T00:00:00.123Z", false, "2026-10-18 00:00:00"},
		{"2026-10-18", false, "2026-10-18 00:00:00"},
		{"2026-10-18", true, "2026-10-18 23:59:59"},
		{"2026-10-18T12:30", false, "2026-10-18 12:30:00"},
		{"  ", false, ""},
		{"garbage", false, "garbage"},
	}
	for _, c := range cases {
		if got := NormalizeTimestamp(c.in, c.endOfDay); got != c.want {
			t.Errorf("NormalizeTimestamp(%q,%v)=%q want %q", c.in, c.endOfDay, got, c.want)
		}
	}
}

func TestSearchMessagesAcceptsISOSinceUntil(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("iso-room", "ISO filters", "", "", "", "", "")
	if _, err := s.PostMessage("iso-room", "bot", "needle", "message", ""); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, since := range []string{
		now.Add(-time.Hour).Format("2006-01-02T15:04:05"),
		now.Add(-time.Hour).Format("2006-01-02T15:04:05Z"),
		now.Add(-time.Hour).Format(time.RFC3339),
		now.Format("2006-01-02"),
	} {
		msgs, err := s.SearchMessages("needle", "", "", "iso-room", "", since, "", 10)
		if err != nil || len(msgs) != 1 {
			t.Errorf("since=%q: got %d msgs, err=%v", since, len(msgs), err)
		}
	}
	until := now.Add(time.Hour).Format("2006-01-02T15:04:05")
	if msgs, _ := s.SearchMessages("needle", "", "", "iso-room", "", "", until, 10); len(msgs) != 1 {
		t.Errorf("until=%q: got %d msgs", until, len(msgs))
	}
	if msgs, _ := s.SearchMessages("needle", "", "", "iso-room", "", "", now.Format("2006-01-02"), 10); len(msgs) != 1 {
		t.Errorf("date-only until should include the whole day, got %d", len(msgs))
	}
}
