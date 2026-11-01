package council

import (
	"database/sql"
	"log/slog"
	"strings"
)

// Typed room links. related_rooms keeps storing plain room IDs (comma-joined, no
// spaces) so every reader of that column, including the Phoenix UI and the
// related_to filter, keeps working unchanged. A link's type, when it is not the
// default "relates", lives in the related_types column as "id=type,id=type".
//
// Reverse rule: when A lists B with a type, B gets A with the inverse type
// (continues <-> continued-by, supersedes <-> superseded-by, blocks <->
// blocked-by); "relates" is its own inverse. A reverse link never overwrites a
// type the target already set, except to upgrade a plain "relates".
const RelatedDefaultType = "relates"

var relatedInverse = map[string]string{
	"relates":       "relates",
	"continues":     "continued-by",
	"continued-by":  "continues",
	"supersedes":    "superseded-by",
	"superseded-by": "supersedes",
	"blocks":        "blocked-by",
	"blocked-by":    "blocks",
}

// RelatedRef is one related-room entry: a room ID and its link type.
type RelatedRef struct {
	ID   string
	Type string
}

// ParseRelated tolerates every spelling clients have been seen to send: a plain
// comma-separated list, a JSON array rendered as a string (["a","b"]), stray
// brackets and quotes, newlines or semicolons as separators, and "type:id"
// entries. Returns clean, de-duplicated refs in first-seen order.
func ParseRelated(raw string) []RelatedRef {
	cleaner := strings.NewReplacer("[", " ", "]", " ", `"`, " ", "'", " ", "\\", " ", "`", " ", "\n", ",", "\r", ",", ";", ",")
	var out []RelatedRef
	index := map[string]int{}
	for _, tok := range strings.Split(cleaner.Replace(raw), ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		typ := RelatedDefaultType
		if i := strings.Index(tok, ":"); i > 0 {
			prefix := strings.ToLower(strings.TrimSpace(tok[:i]))
			if _, ok := relatedInverse[prefix]; ok {
				typ = prefix
				tok = strings.TrimSpace(tok[i+1:])
			}
		}
		if tok == "" {
			continue
		}
		if j, seen := index[tok]; seen {
			if out[j].Type == RelatedDefaultType {
				out[j].Type = typ
			}
			continue
		}
		index[tok] = len(out)
		out = append(out, RelatedRef{ID: tok, Type: typ})
	}
	return out
}

// RelatedColumns renders refs into the two stored columns.
func RelatedColumns(refs []RelatedRef) (ids, types string) {
	var i, t []string
	for _, r := range refs {
		i = append(i, r.ID)
		if r.Type != RelatedDefaultType {
			t = append(t, r.ID+"="+r.Type)
		}
	}
	return strings.Join(i, ","), strings.Join(t, ",")
}

// FormatRelated renders refs for input and display: "continues:a, b".
func FormatRelated(refs []RelatedRef) string {
	parts := make([]string, 0, len(refs))
	for _, r := range refs {
		if r.Type == RelatedDefaultType {
			parts = append(parts, r.ID)
		} else {
			parts = append(parts, r.Type+":"+r.ID)
		}
	}
	return strings.Join(parts, ", ")
}

// RelatedRefs returns the room's related entries with their types.
func (r Room) RelatedRefs() []RelatedRef {
	types := map[string]string{}
	for _, p := range strings.Split(r.RelatedTypes, ",") {
		if k, v, ok := strings.Cut(strings.TrimSpace(p), "="); ok {
			types[k] = v
		}
	}
	refs := ParseRelated(r.RelatedRooms)
	for i := range refs {
		if t, ok := types[refs[i].ID]; ok {
			refs[i].Type = t
		}
	}
	return refs
}

// RelatedDisplay is RelatedRooms with link types shown ("continues:a, b").
func (r Room) RelatedDisplay() string { return FormatRelated(r.RelatedRefs()) }

// MergeRelated adds entries to the room's existing list (nothing removed). A
// typed entry in add overrides an existing plain one.
func MergeRelated(cur Room, add string) string {
	refs := cur.RelatedRefs()
	pos := map[string]int{}
	for i, r := range refs {
		pos[r.ID] = i
	}
	for _, a := range ParseRelated(add) {
		if i, ok := pos[a.ID]; ok {
			if a.Type != RelatedDefaultType {
				refs[i].Type = a.Type
			}
			continue
		}
		pos[a.ID] = len(refs)
		refs = append(refs, a)
	}
	return FormatRelated(refs)
}

// MissingRooms returns the IDs in raw that name no room on this node. Callers
// warn on these; a dangling ID is never an error (the room may live on a peer).
func (s *Server) MissingRooms(raw string) []string {
	var missing []string
	for _, r := range ParseRelated(raw) {
		var one int
		if err := s.DB.QueryRow(`SELECT 1 FROM rooms WHERE id = ?`, r.ID).Scan(&one); err == sql.ErrNoRows {
			missing = append(missing, r.ID)
		}
	}
	return missing
}

// repairRelatedRooms rewrites stored related_rooms values into canonical form
// (clean IDs, comma-joined, types split out). Idempotent; only touches rows
// whose canonical form differs from what is stored.
func repairRelatedRooms(db *sql.DB, logger *slog.Logger) (int, error) {
	rows, err := db.Query(`SELECT id, related_rooms, COALESCE(related_types, '') FROM rooms WHERE related_rooms != ''`)
	if err != nil {
		return 0, err
	}
	type fix struct{ id, ids, types string }
	var fixes []fix
	for rows.Next() {
		var id, rr, rt string
		if rows.Scan(&id, &rr, &rt) != nil {
			continue
		}
		room := Room{RelatedRooms: rr, RelatedTypes: rt}
		ids, types := RelatedColumns(room.RelatedRefs())
		if ids != rr || types != rt {
			fixes = append(fixes, fix{id, ids, types})
		}
	}
	_ = rows.Close()
	for _, f := range fixes {
		if _, err := db.Exec(`UPDATE rooms SET related_rooms = ?, related_types = ? WHERE id = ?`, f.ids, f.types, f.id); err != nil {
			return 0, err
		}
		logger.Info("repaired related_rooms", "room", f.id)
	}
	return len(fixes), nil
}
