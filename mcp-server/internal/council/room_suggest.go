package council

import (
	"sort"
	"strings"
)

// ClosestRoomIDs returns up to limit room ids on this node that look like id:
// a substring match in either direction, or a small edit distance. It reads
// only the id column, so it is cheap enough for an error path.
func (s *Server) ClosestRoomIDs(id string, limit int) []string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" || limit <= 0 {
		return nil
	}
	rows, err := s.DB.Query(`SELECT id FROM rooms LIMIT 5000`)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	type cand struct {
		id    string
		score int
	}
	maxDist := len(id) / 3
	if maxDist < 2 {
		maxDist = 2
	}
	var cands []cand
	for rows.Next() {
		var rid string
		if rows.Scan(&rid) != nil {
			continue
		}
		low := strings.ToLower(rid)
		switch {
		case low == id:
			continue
		case strings.Contains(low, id) || strings.Contains(id, low):
			cands = append(cands, cand{rid, 0})
		default:
			if d := editDistance(low, id); d <= maxDist {
				cands = append(cands, cand{rid, d})
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score < cands[j].score
		}
		return cands[i].id < cands[j].id
	})
	var out []string
	for _, c := range cands {
		if len(out) == limit {
			break
		}
		out = append(out, c.id)
	}
	return out
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
