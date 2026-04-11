package council

import (
	"fmt"
	"regexp"
	"strings"
)

var nonSlugRe = regexp.MustCompile(`[^a-z0-9-]`)

// normalizeTags converts tag strings from various formats to clean CSV.
// Handles JSON array strings like ["a","b"] as well as whitespace-padded CSV.
func normalizeTags(tags string) string {
	if tags == "" {
		return ""
	}
	tags = strings.TrimSpace(tags)
	tags = strings.TrimPrefix(tags, "[")
	tags = strings.TrimSuffix(tags, "]")
	tags = strings.ReplaceAll(tags, `"`, "")
	var parts []string
	for _, t := range strings.Split(tags, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, ",")
}

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
	tags = normalizeTags(tags)

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
		args = append(args, normalizeTags(tags))
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
