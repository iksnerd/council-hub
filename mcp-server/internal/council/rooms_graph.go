package council

import (
	"fmt"
	"strings"
)

// ConceptMapNode represents a room and its depth in a conceptual graph.
type ConceptMapNode struct {
	Room     Room
	Depth    int
	Via      string // ID of the room that linked to this one
	Inferred string // non-empty when relationship was inferred: "project" or "tags: foo,bar"
}

// GetConceptMap traverses the room graph starting from startID up to maxDepth using BFS.
// inferFrom may be "project", "tags", "project,tags", or "" for explicit links only.
// Inferred neighbors are included alongside explicit related_rooms at each hop.
func (s *Server) GetConceptMap(startID string, maxDepth int, inferFrom string) ([]ConceptMapNode, error) {
	if maxDepth < 0 {
		maxDepth = 3
	}
	if maxDepth > 5 {
		maxDepth = 5
	}

	inferProject := strings.Contains(inferFrom, "project")
	inferTags := strings.Contains(inferFrom, "tags")

	s.Mu.RLock()
	defer s.Mu.RUnlock()

	if _, err := s.GetRoom(startID); err != nil {
		return nil, fmt.Errorf("start room '%s' not found: %w", startID, err)
	}

	type queueItem struct {
		id       string
		depth    int
		via      string
		inferred string
	}

	var results []ConceptMapNode
	visited := make(map[string]bool)
	queue := []queueItem{{id: startID, depth: 0}}
	visited[startID] = true

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		room, err := s.GetRoom(item.id)
		if err != nil {
			continue
		}

		results = append(results, ConceptMapNode{
			Room:     room,
			Depth:    item.depth,
			Via:      item.via,
			Inferred: item.inferred,
		})

		if item.depth >= maxDepth {
			continue
		}

		// Explicit related_rooms links.
		for _, relID := range strings.Split(room.RelatedRooms, ",") {
			relID = strings.TrimSpace(relID)
			if relID != "" && !visited[relID] {
				visited[relID] = true
				queue = append(queue, queueItem{id: relID, depth: item.depth + 1, via: item.id})
			}
		}

		// Inferred: same-project rooms.
		if inferProject && room.Project != "" {
			rows, err := s.DB.Query(`SELECT id FROM rooms WHERE project = ? AND id != ?`, room.Project, room.ID)
			if err == nil {
				for rows.Next() {
					var id string
					if rows.Scan(&id) == nil && !visited[id] {
						visited[id] = true
						queue = append(queue, queueItem{id: id, depth: item.depth + 1, via: item.id, inferred: "project"})
					}
				}
				_ = rows.Close()
			}
		}

		// Inferred: rooms sharing any tag.
		if inferTags && room.Tags != "" {
			sharedTags := make(map[string][]string) // neighbor id -> matching tags
			for _, tag := range strings.Split(room.Tags, ",") {
				tag = strings.TrimSpace(tag)
				if tag == "" {
					continue
				}
				rows, err := s.DB.Query(
					`SELECT id FROM rooms WHERE id != ? AND (',' || tags || ',') LIKE '%,' || ? || ',%'`,
					room.ID, tag,
				)
				if err != nil {
					continue
				}
				for rows.Next() {
					var id string
					if rows.Scan(&id) == nil {
						sharedTags[id] = append(sharedTags[id], tag)
					}
				}
				_ = rows.Close()
			}
			for id, tags := range sharedTags {
				if !visited[id] {
					visited[id] = true
					queue = append(queue, queueItem{
						id:       id,
						depth:    item.depth + 1,
						via:      item.id,
						inferred: "tags: " + strings.Join(tags, ","),
					})
				}
			}
		}
	}

	return results, nil
}
