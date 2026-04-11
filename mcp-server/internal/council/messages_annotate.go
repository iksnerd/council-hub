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
	var actualRoomID string
	err := s.DB.QueryRow(`SELECT room_id, pinned FROM messages WHERE id = ?`, messageID).Scan(&actualRoomID, &currentlyPinned)
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

	// Unpin any existing pinned message in this room
	_, _ = s.DB.Exec(`UPDATE messages SET pinned = 0 WHERE room_id = ? AND pinned = 1`, roomID)

	// Pin the target
	_, err = s.DB.Exec(`UPDATE messages SET pinned = 1 WHERE id = ?`, messageID)
	if err != nil {
		return false, err
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
