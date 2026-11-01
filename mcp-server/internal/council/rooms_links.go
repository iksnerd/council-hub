package council

// syncReverseLinks ensures that if room A lists B in related_rooms, B also lists
// A, with the inverse link type (see related.go).
// Must be called while s.Mu is held.
func (s *Server) syncReverseLinks(roomID string, refs []RelatedRef) {
	for _, ref := range refs {
		if ref.ID == "" || ref.ID == roomID {
			continue
		}
		var target Room
		err := s.DB.QueryRow(`SELECT related_rooms, COALESCE(related_types, '') FROM rooms WHERE id = ?`, ref.ID).
			Scan(&target.RelatedRooms, &target.RelatedTypes)
		if err != nil {
			continue // target room doesn't exist, skip
		}
		inverse := relatedInverse[ref.Type]
		existing := target.RelatedRefs()
		changed := false
		found := false
		for i := range existing {
			if existing[i].ID == roomID {
				found = true
				if existing[i].Type == RelatedDefaultType && inverse != RelatedDefaultType {
					existing[i].Type = inverse
					changed = true
				}
				break
			}
		}
		if !found {
			existing = append(existing, RelatedRef{ID: roomID, Type: inverse})
			changed = true
		}
		if !changed {
			continue
		}
		ids, types := RelatedColumns(existing)
		if _, err := s.DB.Exec(`UPDATE rooms SET related_rooms = ?, related_types = ? WHERE id = ?`, ids, types, ref.ID); err != nil {
			s.Logger.Warn("syncReverseLinks: failed to update related_rooms", "room", ref.ID, "error", err)
		}
	}
}

// removeRoomFromRelatedLinks removes roomID from every other room's related_rooms field.
// Must be called while s.Mu is held.
func (s *Server) removeRoomFromRelatedLinks(roomID string) {
	rows, err := s.DB.Query(
		`SELECT id, related_rooms, COALESCE(related_types, '') FROM rooms WHERE related_rooms LIKE '%' || ? || '%'`,
		roomID,
	)
	if err != nil {
		s.Logger.Warn("removeRoomFromRelatedLinks: query failed", "error", err)
		return
	}
	type update struct{ id, ids, types string }
	var updates []update
	for rows.Next() {
		var id string
		var r Room
		if err := rows.Scan(&id, &r.RelatedRooms, &r.RelatedTypes); err != nil {
			continue
		}
		var kept []RelatedRef
		for _, ref := range r.RelatedRefs() {
			if ref.ID != roomID {
				kept = append(kept, ref)
			}
		}
		ids, types := RelatedColumns(kept)
		updates = append(updates, update{id, ids, types})
	}
	_ = rows.Close()
	for _, u := range updates {
		if _, err := s.DB.Exec(`UPDATE rooms SET related_rooms = ?, related_types = ? WHERE id = ?`, u.ids, u.types, u.id); err != nil {
			s.Logger.Warn("removeRoomFromRelatedLinks: update failed", "room", u.id, "error", err)
		}
	}
}
