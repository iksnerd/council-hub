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
	rows.Close()
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

func (s *Server) UpdateRoom(roomID, description, project, techStack, tags, systemPrompt, relatedRooms string) error {
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

	if _, err := s.DB.Exec(`DELETE FROM messages WHERE room_id = ?`, roomID); err != nil {
		return fmt.Errorf("delete messages for room '%s': %w", roomID, err)
	}
	return nil
}

// ListRooms returns rooms matching optional filters.
func (s *Server) ListRooms(project, tag, status, search string) ([]Room, error) {
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

	query += ` ORDER BY updated_at DESC`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
