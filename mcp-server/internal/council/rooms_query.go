package council

import "strings"

// ListRooms returns rooms matching optional filters.
func (s *Server) ListRooms(project, tag, status, search string, limit, offset int) ([]Room, error) {
	query := `SELECT id, description, status, project, tech_stack, tags, system_prompt, related_rooms, created_at, updated_at FROM rooms WHERE 1=1`
	var args []any

	if project != "" {
		query += ` AND project = ?`
		args = append(args, normalizeProject(project))
	}
	if tag != "" {
		query += ` AND (',' || tags || ',') LIKE '%,' || ? || ',%'`
		args = append(args, tag)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if search != "" {
		for _, word := range strings.Fields(search) {
			query += ` AND (id LIKE '%' || ? || '%' OR description LIKE '%' || ? || '%' OR tags LIKE '%' || ? || '%')`
			args = append(args, word, word, word)
		}
	}

	query += ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`

	if limit <= 0 {
		limit = 50 // default to 50
	} else if limit > 100 {
		limit = 100 // cap at 100
	}
	if offset < 0 {
		offset = 0
	}

	args = append(args, limit, offset)

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.Description, &r.Status, &r.Project, &r.TechStack, &r.Tags, &r.SystemPrompt, &r.RelatedRooms, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

// SimilarRoom represents a room that may overlap with a newly created one.
type SimilarRoom struct {
	ID          string
	Description string
	Tags        string
	Project     string
	MatchReason string
}

// stopWords are short/common words excluded from description keyword matching.
var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true,
	"this": true, "are": true, "was": true, "from": true, "have": true,
}

// FindSimilarRooms returns rooms that overlap with the given metadata.
// excludeID is the room to exclude (typically the one just created).
// Returns at most limit results, sorted by similarity score descending.
// Errors are non-fatal — callers should silently discard them.
func (s *Server) FindSimilarRooms(excludeID, description, project, tags string, limit int) ([]SimilarRoom, error) {
	// Parse input tags into a set.
	inputTags := map[string]bool{}
	for _, t := range strings.Split(tags, ",") {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" {
			inputTags[t] = true
		}
	}

	// Extract significant words from description (len>=4, not stop words).
	inputWords := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(description)) {
		w = strings.Trim(w, ".,;:!?\"'()")
		if len(w) >= 4 && !stopWords[w] {
			inputWords[w] = true
		}
	}

	// If no useful signal, skip the query.
	if len(inputTags) == 0 && len(inputWords) == 0 {
		return nil, nil
	}

	rooms, err := s.ListRooms(project, "", "active", "", 100, 0)
	if err != nil {
		return nil, err
	}

	type scored struct {
		room   Room
		score  int
		reason []string
	}

	var candidates []scored
	for _, r := range rooms {
		if r.ID == excludeID {
			continue
		}

		sc := scored{room: r}

		// Tag overlap.
		for _, t := range strings.Split(r.Tags, ",") {
			t = strings.TrimSpace(strings.ToLower(t))
			if t != "" && inputTags[t] {
				sc.score += 2
				sc.reason = append(sc.reason, "tag:"+t)
			}
		}

		// Description keyword overlap.
		for _, w := range strings.Fields(strings.ToLower(r.Description)) {
			w = strings.Trim(w, ".,;:!?\"'()")
			if len(w) >= 4 && inputWords[w] {
				sc.score++
				sc.reason = append(sc.reason, "topic:"+w)
			}
		}

		if sc.score >= 3 {
			candidates = append(candidates, sc)
		}
	}

	// Sort by score descending.
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].score > candidates[j-1].score; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := make([]SimilarRoom, len(candidates))
	for i, c := range candidates {
		// Summarise reason: shared tags first, then keywords.
		var tagParts, words []string
		seen := map[string]bool{}
		for _, r := range c.reason {
			if seen[r] {
				continue
			}
			seen[r] = true
			if strings.HasPrefix(r, "tag:") {
				tagParts = append(tagParts, strings.TrimPrefix(r, "tag:"))
			} else {
				words = append(words, strings.TrimPrefix(r, "topic:"))
			}
		}
		var parts []string
		if len(tagParts) > 0 {
			parts = append(parts, "shared tags: "+strings.Join(tagParts, ", "))
		}
		if len(words) > 0 {
			parts = append(parts, "similar topic: "+strings.Join(words, ", "))
		}
		result[i] = SimilarRoom{
			ID:          c.room.ID,
			Description: c.room.Description,
			Tags:        c.room.Tags,
			Project:     c.room.Project,
			MatchReason: strings.Join(parts, "; "),
		}
	}
	return result, nil
}
