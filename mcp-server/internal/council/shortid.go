package council

import "sort"

// minShortIDLen is the shortest id prefix any display surface prints. It matches
// minResolvePrefixLen, the shortest prefix ResolveMessageID accepts.
const minShortIDLen = minResolvePrefixLen

// ShortIDs maps full message ids to the shortest prefix that is unique within
// the set it was built from, never shorter than minShortIDLen. UUIDv7 ids
// posted in the same second share their first 8 characters, so a fixed 8-char
// prefix can name two messages; this extends only where it must.
//
// Build it from every id in a room (RoomShortIDs), not from a filtered view:
// the resolver sees all of them, including revised ones a transcript hides.
type ShortIDs struct {
	short map[string]string
}

// NewShortIDs builds the mapping for ids. Duplicates are ignored.
func NewShortIDs(ids []string) *ShortIDs {
	sorted := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			sorted = append(sorted, id)
		}
	}
	sort.Strings(sorted)
	out := &ShortIDs{short: make(map[string]string, len(sorted))}
	for i, id := range sorted {
		n := minShortIDLen
		if i > 0 {
			if c := commonPrefixLen(sorted[i-1], id) + 1; c > n {
				n = c
			}
		}
		if i+1 < len(sorted) {
			if c := commonPrefixLen(id, sorted[i+1]) + 1; c > n {
				n = c
			}
		}
		if n > len(id) {
			n = len(id)
		}
		out.short[id] = id[:n]
	}
	return out
}

// Of returns the display prefix for id. An id outside the set (or a nil
// receiver) gets the plain 8-char prefix.
func (s *ShortIDs) Of(id string) string {
	if s != nil {
		if p, ok := s.short[id]; ok {
			return p
		}
	}
	if len(id) > minShortIDLen {
		return id[:minShortIDLen]
	}
	return id
}

func commonPrefixLen(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

// RoomShortIDs builds ShortIDs over every message id in roomID, revised and
// retracted ones included.
func (s *Server) RoomShortIDs(roomID string) (*ShortIDs, error) {
	rows, err := s.DB.Query(`SELECT id FROM messages WHERE room_id = ?`, roomID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return NewShortIDs(ids), nil
}

// ShortIDsOrDefault is RoomShortIDs that degrades to the 8-char prefix on a
// query error, so a display path never fails over cosmetics.
func (s *Server) ShortIDsOrDefault(roomID string) *ShortIDs {
	ids, err := s.RoomShortIDs(roomID)
	if err != nil {
		return nil
	}
	return ids
}
