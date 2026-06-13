package council

import (
	"strings"
	"time"
)

// ViewSpec is a composable specification for how a transcript is projected before
// rendering — Council Hub's take on Engelbart's NLS ViewSpecs. The default shows
// everything (byte-identical to the un-specified render); flags narrow the view.
//
// This first slice covers the metadata toggles (NLS I/N — show/hide IDs, author,
// timestamps, reactions) and line-clipping (NLS T — first line of each message).
// Structural dimensions (level-clip L, link-distance) land once they ride the link
// graph and notebook outlines.
type ViewSpec struct {
	ShowIDs         bool
	ShowAuthor      bool
	ShowTimestamps  bool
	ShowReactions   bool
	TruncateLineOne bool
}

// DefaultViewSpec renders the full transcript — every metadata field, full bodies.
func DefaultViewSpec() ViewSpec {
	return ViewSpec{ShowIDs: true, ShowAuthor: true, ShowTimestamps: true, ShowReactions: true}
}

// ParseViewSpec builds a ViewSpec from the tool params. `show` is a comma list
// selecting which metadata to include (tokens: ids, author, time, reactions) —
// when empty, all are shown. `truncate` of "line-one" clips each body to its first
// line. Unknown tokens are ignored.
func ParseViewSpec(show, truncate string) ViewSpec {
	v := DefaultViewSpec()

	show = strings.TrimSpace(show)
	if show != "" {
		// An explicit selection means "show only these" — start from all-off.
		v.ShowIDs, v.ShowAuthor, v.ShowTimestamps, v.ShowReactions = false, false, false, false
		for _, tok := range strings.Split(show, ",") {
			switch strings.ToLower(strings.TrimSpace(tok)) {
			case "ids", "id":
				v.ShowIDs = true
			case "author", "authors":
				v.ShowAuthor = true
			case "time", "timestamps", "timestamp", "ts":
				v.ShowTimestamps = true
			case "reactions", "reaction":
				v.ShowReactions = true
			}
		}
	}

	switch strings.ToLower(strings.TrimSpace(truncate)) {
	case "line-one", "line1", "first-line", "one":
		v.TruncateLineOne = true
	}
	return v
}

// firstLine returns the text up to the first newline, with an ellipsis when it clipped.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimRight(s[:i], " \t") + " …"
	}
	return s
}

// parseFilterTime accepts "2026-06-01", "2026-06-01 12:30:00", or the ISO "T" form.
func parseFilterTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "T", " "))
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// FilterMessages narrows a transcript to messages matching the given criteria — the
// "which nodes" half of a ViewSpec (the rendering half is ViewSpec itself). Author is
// matched case-insensitively as a substring (so "claude" matches "Claude Code (Opus)");
// type is exact; since/until bound the timestamp. Empty criteria pass everything through.
func FilterMessages(msgs []Message, author, msgType, since, until string) []Message {
	author = strings.ToLower(strings.TrimSpace(author))
	msgType = strings.TrimSpace(msgType)
	sinceT, hasSince := parseFilterTime(since)
	untilT, hasUntil := parseFilterTime(until)

	if author == "" && msgType == "" && !hasSince && !hasUntil {
		return msgs
	}

	var out []Message
	for _, m := range msgs {
		if author != "" && !strings.Contains(strings.ToLower(m.Author), author) {
			continue
		}
		if msgType != "" && m.MessageType != msgType {
			continue
		}
		if hasSince && m.Timestamp.Before(sinceT) {
			continue
		}
		if hasUntil && m.Timestamp.After(untilT) {
			continue
		}
		out = append(out, m)
	}
	return out
}
