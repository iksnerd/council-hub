package council

import (
	"strings"
	"time"
)

// storedTimeLayout is how messages.timestamp is stored (UTC, space separator).
// Filters compare it as a string, so every since/until must be normalised to it.
const storedTimeLayout = "2006-01-02 15:04:05"

// NormalizeTimestamp turns a user-supplied since/until value into the stored
// format. It accepts the ISO "T" form, a trailing Z or numeric offset (converted
// to UTC), fractional seconds, "YYYY-MM-DD HH:MM[:SS]" and date-only. A date-only
// value means 00:00:00, or 23:59:59 when endOfDay is set (use for until).
// Unparseable input is returned with only the T separator replaced, so a typo
// still fails the filter rather than silently matching everything. Empty stays empty.
func NormalizeTimestamp(s string, endOfDay bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05Z0700", "2006-01-02T15:04:05.999999999Z0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(storedTimeLayout)
		}
	}
	sp := strings.ReplaceAll(s, "T", " ")
	for _, layout := range []string{"2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if t, err := time.Parse(layout, sp); err == nil {
			return t.Format(storedTimeLayout)
		}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		if endOfDay {
			return t.Format("2006-01-02") + " 23:59:59"
		}
		return t.Format(storedTimeLayout)
	}
	return sp
}
