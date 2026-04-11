package council

import (
	"fmt"
	"strings"
)

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
