package council

import "fmt"

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
