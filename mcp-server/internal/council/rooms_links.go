package council

import "strings"

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
