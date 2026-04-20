package council

import "fmt"

// RenameProject rewrites the project field on every room currently assigned
// to the `from` project, replacing it with `to`. Both names are normalized
// the same way as create_room/update_room writes, so callers don't have to
// pre-slugify. Returns the number of rooms updated.
func (s *Server) RenameProject(from, to string) (int, error) {
	from = normalizeProject(from)
	to = normalizeProject(to)
	if from == "" || to == "" {
		return 0, fmt.Errorf("both 'from' and 'to' project names are required")
	}
	if from == to {
		return 0, nil
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	res, err := s.DB.Exec(
		`UPDATE rooms SET project = ?, updated_at = CURRENT_TIMESTAMP WHERE project = ?`,
		to, from,
	)
	if err != nil {
		return 0, fmt.Errorf("rename project '%s' -> '%s': %w", from, to, err)
	}
	rows, _ := res.RowsAffected()
	return int(rows), nil
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

	// Strip health tags when resolving — resolved rooms are skipped by check_room_health
	// so these tags would otherwise persist forever and pollute tag filters.
	if status == "resolved" {
		var currentTags string
		if err := s.DB.QueryRow(`SELECT COALESCE(tags, '') FROM rooms WHERE id = ?`, roomID).Scan(&currentTags); err == nil {
			newTags := removeTag(removeTag(currentTags, "needs-synthesis"), "stale")
			if newTags != currentTags {
				_, _ = s.DB.Exec(`UPDATE rooms SET tags = ? WHERE id = ?`, newTags, roomID)
			}
		}
	}

	return nil
}
