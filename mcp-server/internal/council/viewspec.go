package council

import "strings"

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
