package council

import (
	"encoding/json"
	"fmt"
)

func (s *Server) PinMessage(roomID string, messageID string) (bool, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Verify message exists and belongs to the room
	var currentlyPinned bool
	var actualRoomID, targetType, targetSupersedes string
	err := s.DB.QueryRow(`SELECT room_id, pinned, message_type, COALESCE(supersedes, '') FROM messages WHERE id = ?`, messageID).
		Scan(&actualRoomID, &currentlyPinned, &targetType, &targetSupersedes)
	if err != nil {
		return false, err
	}
	if actualRoomID != roomID {
		return false, fmt.Errorf("message %.8s belongs to room '%s', not '%s'", messageID, actualRoomID, roomID)
	}

	if currentlyPinned {
		// Toggle off
		_, err := s.DB.Exec(`UPDATE messages SET pinned = 0 WHERE id = ?`, messageID)
		return false, err
	}

	// Capture the currently pinned message (if any) before we unpin it — used to
	// auto-link a synthesis-replacing-a-synthesis with supersedes.
	var oldPinID, oldPinType string
	_ = s.DB.QueryRow(`SELECT id, message_type FROM messages WHERE room_id = ? AND pinned = 1 LIMIT 1`, roomID).
		Scan(&oldPinID, &oldPinType)

	// Unpin any existing pinned message in this room
	_, _ = s.DB.Exec(`UPDATE messages SET pinned = 0 WHERE room_id = ? AND pinned = 1`, roomID)

	// Pin the target
	_, err = s.DB.Exec(`UPDATE messages SET pinned = 1 WHERE id = ?`, messageID)
	if err != nil {
		return false, err
	}

	// Pin-replacement chaining: when a synthesis replaces a previously pinned
	// synthesis and doesn't already declare what it supersedes, record the link.
	if oldPinID != "" && oldPinID != messageID &&
		oldPinType == "synthesis" && targetType == "synthesis" && targetSupersedes == "" {
		_, _ = s.DB.Exec(`UPDATE messages SET supersedes = ? WHERE id = ?`, oldPinID, messageID)
	}

	// A fresh pin clears any `stale-pin` flag the linter set on the room.
	var tags string
	if err := s.DB.QueryRow(`SELECT COALESCE(tags, '') FROM rooms WHERE id = ?`, roomID).Scan(&tags); err == nil {
		if newTags := removeTag(tags, "stale-pin"); newTags != tags {
			_, _ = s.DB.Exec(`UPDATE rooms SET tags = ? WHERE id = ?`, newTags, roomID)
		}
	}

	return true, nil
}

// ReactToMessage toggles an emoji reaction by an author on a message.
// Returns the updated reactions map and whether the reaction was added (true) or removed (false).
func (s *Server) ReactToMessage(messageID, emoji, author string) (map[string][]string, bool, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	var reactionsJSON string
	var actualRoomID string
	err := s.DB.QueryRow(`SELECT room_id, reactions FROM messages WHERE id = ?`, messageID).Scan(&actualRoomID, &reactionsJSON)
	if err != nil {
		return nil, false, fmt.Errorf("message '%.8s' not found", messageID)
	}

	reactions := make(map[string][]string)
	if reactionsJSON != "" && reactionsJSON != "{}" {
		if err := json.Unmarshal([]byte(reactionsJSON), &reactions); err != nil {
			reactions = make(map[string][]string)
		}
	}

	// Toggle: remove if already present, add otherwise
	added := true
	authors := reactions[emoji]
	found := -1
	for i, a := range authors {
		if a == author {
			found = i
			break
		}
	}
	if found >= 0 {
		reactions[emoji] = append(authors[:found], authors[found+1:]...)
		if len(reactions[emoji]) == 0 {
			delete(reactions, emoji)
		}
		added = false
	} else {
		reactions[emoji] = append(authors, author)
	}

	out, _ := json.Marshal(reactions)
	_, err = s.DB.Exec(`UPDATE messages SET reactions = ? WHERE id = ?`, string(out), messageID)
	if err != nil {
		return nil, false, err
	}

	return reactions, added, nil
}
