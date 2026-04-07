package council

import (
	"fmt"
	"regexp"
	"strings"
)

var nonSlugRe = regexp.MustCompile(`[^a-z0-9-]`)

// normalizeProject converts a project name to a URL-safe slug: lowercase,
// spaces/underscores become hyphens, non-alphanumeric chars stripped,
// consecutive hyphens collapsed, leading/trailing hyphens trimmed.
func normalizeProject(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = nonSlugRe.ReplaceAllString(s, "")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// ConceptMapNode represents a room and its depth in a conceptual graph.
type ConceptMapNode struct {
	Room  Room
	Depth int
	Via   string // ID of the room that linked to this one
}

// GetConceptMap traverses the room graph starting from startID up to maxDepth using BFS.
func (s *Server) GetConceptMap(startID string, maxDepth int) ([]ConceptMapNode, error) {
	if maxDepth < 0 {
		maxDepth = 3
	}
	if maxDepth > 5 {
		maxDepth = 5 // Limit depth to prevent runaway traversal
	}

	s.Mu.RLock()
	defer s.Mu.RUnlock()

	if _, err := s.GetRoom(startID); err != nil {
		return nil, fmt.Errorf("start room '%s' not found: %w", startID, err)
	}

	type queueItem struct {
		id    string
		depth int
		via   string
	}

	var results []ConceptMapNode
	visited := make(map[string]bool)
	queue := []queueItem{{id: startID, depth: 0, via: ""}}
	visited[startID] = true

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		room, err := s.GetRoom(item.id)
		if err != nil {
			// Room might have been deleted mid-traversal or target doesn't exist
			continue
		}

		results = append(results, ConceptMapNode{
			Room:  room,
			Depth: item.depth,
			Via:   item.via,
		})

		if item.depth < maxDepth {
			for _, relID := range strings.Split(room.RelatedRooms, ",") {
				relID = strings.TrimSpace(relID)
				if relID != "" && !visited[relID] {
					visited[relID] = true
					queue = append(queue, queueItem{
						id:    relID,
						depth: item.depth + 1,
						via:   item.id,
					})
				}
			}
		}
	}

	return results, nil
}

func (s *Server) CreateRoom(id, description, project, techStack, tags, systemPrompt, relatedRooms string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	project = normalizeProject(project)

	_, err := s.DB.Exec(
		`INSERT OR IGNORE INTO rooms (id, description, project, tech_stack, tags, system_prompt, related_rooms) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, description, project, techStack, tags, systemPrompt, relatedRooms,
	)
	if err != nil {
		return err
	}

	s.syncReverseLinks(id, relatedRooms)

	// Embed room description + system prompt (non-fatal)
	text := description
	if systemPrompt != "" {
		text += " " + systemPrompt
	}
	s.EmbedAsync("room_vectors", id, text)

	return nil
}

// syncReverseLinks ensures that if room A lists B in related_rooms, B also lists A.
// Must be called while s.Mu is held.
func (s *Server) syncReverseLinks(roomID, relatedRooms string) {
	if relatedRooms == "" {
		return
	}
	for _, rel := range strings.Split(relatedRooms, ",") {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		var existing string
		err := s.DB.QueryRow(`SELECT related_rooms FROM rooms WHERE id = ?`, rel).Scan(&existing)
		if err != nil {
			continue // target room doesn't exist, skip
		}
		// Check if roomID is already in the target's related_rooms
		already := false
		for _, r := range strings.Split(existing, ",") {
			if strings.TrimSpace(r) == roomID {
				already = true
				break
			}
		}
		if !already {
			updated := existing
			if updated == "" {
				updated = roomID
			} else {
				updated = updated + ", " + roomID
			}
			if _, err := s.DB.Exec(`UPDATE rooms SET related_rooms = ? WHERE id = ?`, updated, rel); err != nil {
				s.Logger.Warn("syncReverseLinks: failed to update related_rooms", "room", rel, "error", err)
			}
		}
	}
}

// removeRoomFromRelatedLinks removes roomID from every other room's related_rooms field.
// Must be called while s.Mu is held.
func (s *Server) removeRoomFromRelatedLinks(roomID string) {
	rows, err := s.DB.Query(
		`SELECT id, related_rooms FROM rooms WHERE related_rooms LIKE '%' || ? || '%'`,
		roomID,
	)
	if err != nil {
		s.Logger.Warn("removeRoomFromRelatedLinks: query failed", "error", err)
		return
	}
	type update struct{ id, newRR string }
	var updates []update
	for rows.Next() {
		var id, rr string
		if err := rows.Scan(&id, &rr); err != nil {
			continue
		}
		var kept []string
		for _, rel := range strings.Split(rr, ",") {
			rel = strings.TrimSpace(rel)
			if rel != "" && rel != roomID {
				kept = append(kept, rel)
			}
		}
		updates = append(updates, update{id, strings.Join(kept, ", ")})
	}
	_ = rows.Close()
	for _, u := range updates {
		if _, err := s.DB.Exec(`UPDATE rooms SET related_rooms = ? WHERE id = ?`, u.newRR, u.id); err != nil {
			s.Logger.Warn("removeRoomFromRelatedLinks: update failed", "room", u.id, "error", err)
		}
	}
}

func (s *Server) UpdateStatus(roomID, status string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	res, err := s.DB.Exec(
		`UPDATE rooms SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, roomID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("room '%s' not found", roomID)
	}
	return nil
}

func (s *Server) UpdateRoom(roomID, description, project, techStack, tags, addTags, removeTags, systemPrompt, relatedRooms string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Build dynamic UPDATE — only set fields that are non-empty.
	setClauses := []string{"updated_at = CURRENT_TIMESTAMP"}
	var args []any

	if description != "" {
		setClauses = append(setClauses, "description = ?")
		args = append(args, description)
	}
	if project != "" {
		project = normalizeProject(project)
		setClauses = append(setClauses, "project = ?")
		args = append(args, project)
	}
	if techStack != "" {
		setClauses = append(setClauses, "tech_stack = ?")
		args = append(args, techStack)
	}

	if tags != "" {
		setClauses = append(setClauses, "tags = ?")
		args = append(args, tags)
	} else if addTags != "" || removeTags != "" {
		// Fetch current tags to modify them
		var currentTags string
		_ = s.DB.QueryRow(`SELECT COALESCE(tags, '') FROM rooms WHERE id = ?`, roomID).Scan(&currentTags)

		tagMap := make(map[string]bool)
		for _, t := range strings.Split(currentTags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagMap[t] = true
			}
		}
		for _, t := range strings.Split(addTags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagMap[t] = true
			}
		}
		for _, t := range strings.Split(removeTags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				delete(tagMap, t)
			}
		}
		var newTags []string
		for t := range tagMap {
			newTags = append(newTags, t)
		}
		setClauses = append(setClauses, "tags = ?")
		args = append(args, strings.Join(newTags, ","))
	}

	if systemPrompt != "" {
		setClauses = append(setClauses, "system_prompt = ?")
		args = append(args, systemPrompt)
	}
	if relatedRooms != "" {
		setClauses = append(setClauses, "related_rooms = ?")
		args = append(args, relatedRooms)
	}

	query := fmt.Sprintf("UPDATE rooms SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	args = append(args, roomID)

	res, err := s.DB.Exec(query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("room '%s' not found", roomID)
	}

	s.syncReverseLinks(roomID, relatedRooms)

	// Re-embed if description or system_prompt changed (non-fatal)
	if description != "" || systemPrompt != "" {
		room, err := s.GetRoom(roomID)
		if err == nil {
			text := room.Description
			if room.SystemPrompt != "" {
				text += " " + room.SystemPrompt
			}
			s.EmbedAsync("room_vectors", roomID, text)
		}
	}

	return nil
}

func (s *Server) GetRoom(roomID string) (Room, error) {
	var r Room
	err := s.DB.QueryRow(
		`SELECT id, description, status, project, tech_stack, tags, system_prompt, related_rooms, created_at, updated_at FROM rooms WHERE id = ?`,
		roomID,
	).Scan(&r.ID, &r.Description, &r.Status, &r.Project, &r.TechStack, &r.Tags, &r.SystemPrompt, &r.RelatedRooms, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return r, err
	}
	return r, nil
}

func (s *Server) DeleteRoom(roomID string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	s.removeRoomFromRelatedLinks(roomID)

	res, err := s.DB.Exec(`DELETE FROM rooms WHERE id = ?`, roomID)
	if err != nil {
		return fmt.Errorf("delete room '%s': %w", roomID, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("room '%s' not found", roomID)
	}

	// Clean up vectors before deleting messages (best-effort)
	_, _ = s.DB.Exec(`DELETE FROM message_vectors WHERE message_id IN (SELECT id FROM messages WHERE room_id = ?)`, roomID)
	_, _ = s.DB.Exec(`DELETE FROM room_vectors WHERE room_id = ?`, roomID)

	if _, err := s.DB.Exec(`DELETE FROM messages WHERE room_id = ?`, roomID); err != nil {
		return fmt.Errorf("delete messages for room '%s': %w", roomID, err)
	}

	return nil
}

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
		var tags, words []string
		seen := map[string]bool{}
		for _, r := range c.reason {
			if seen[r] {
				continue
			}
			seen[r] = true
			if strings.HasPrefix(r, "tag:") {
				tags = append(tags, strings.TrimPrefix(r, "tag:"))
			} else {
				words = append(words, strings.TrimPrefix(r, "topic:"))
			}
		}
		var parts []string
		if len(tags) > 0 {
			parts = append(parts, "shared tags: "+strings.Join(tags, ", "))
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
