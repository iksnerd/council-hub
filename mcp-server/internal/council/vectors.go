package council

import (
	"context"
	"fmt"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func init() {
	sqlite_vec.Auto()
}

// StoreVector upserts a vector into the given table (message_vectors or room_vectors).
// Acquires its own lock — do not call while holding s.Mu.
func (s *Server) StoreVector(table, id string, vec []float32) error {
	idCol := "message_id"
	if table == "room_vectors" {
		idCol = "room_id"
	}

	serialized, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return fmt.Errorf("serialize vector: %w", err)
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Delete existing vector if present (upsert)
	_, _ = s.DB.Exec(fmt.Sprintf(`DELETE FROM %s WHERE %s = ?`, table, idCol), id)

	_, err = s.DB.Exec(
		fmt.Sprintf(`INSERT INTO %s(%s, embedding) VALUES (?, ?)`, table, idCol),
		id, serialized,
	)
	return err
}

// deleteVectorsLocked removes multiple vectors. Caller MUST hold s.Mu.
func (s *Server) deleteVectorsLocked(table string, ids []string) {
	if len(ids) == 0 {
		return
	}
	idCol := "message_id"
	if table == "room_vectors" {
		idCol = "room_id"
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	_, _ = s.DB.Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE %s IN (%s)`, table, idCol, strings.Join(placeholders, ",")),
		args...,
	)
}

// SearchMessagesSemantic finds messages similar to the query vector.
func (s *Server) SearchMessagesSemantic(query string, roomID, project, author, messageType, since, until string, limit int) ([]Message, error) {
	if s.Embedder == nil {
		return nil, fmt.Errorf("semantic search unavailable: no embedder configured")
	}

	queryVec, err := s.Embedder.Embed(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	serialized, err := sqlite_vec.SerializeFloat32(queryVec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}

	// First: get candidate message IDs from vector search (wider net)
	candidateLimit := limit * 3
	vecRows, err := s.DB.Query(
		`SELECT message_id, distance FROM message_vectors WHERE embedding MATCH ? ORDER BY distance LIMIT ?`,
		serialized, candidateLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer vecRows.Close()

	var candidateIDs []string
	distances := make(map[string]float64)
	for vecRows.Next() {
		var id string
		var dist float64
		if err := vecRows.Scan(&id, &dist); err != nil {
			continue
		}
		candidateIDs = append(candidateIDs, id)
		distances[id] = dist
	}

	if len(candidateIDs) == 0 {
		return nil, nil
	}

	// Second: fetch full messages with filters applied
	placeholders := make([]string, len(candidateIDs))
	args := make([]any, len(candidateIDs))
	for i, id := range candidateIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	where := fmt.Sprintf(`WHERE m.id IN (%s)`, strings.Join(placeholders, ","))

	if roomID != "" {
		parts := strings.Split(roomID, ",")
		if len(parts) == 1 {
			where += ` AND m.room_id = ?`
			args = append(args, strings.TrimSpace(parts[0]))
		} else {
			rPlaceholders := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					rPlaceholders = append(rPlaceholders, "?")
					args = append(args, p)
				}
			}
			if len(rPlaceholders) > 0 {
				where += ` AND m.room_id IN (` + strings.Join(rPlaceholders, ",") + `)`
			}
		}
	}
	if author != "" {
		where += ` AND m.author = ?`
		args = append(args, author)
	}
	if messageType != "" {
		where += ` AND m.message_type = ?`
		args = append(args, messageType)
	}

	join := ""
	if project != "" {
		join = ` JOIN rooms r ON m.room_id = r.id`
		where += ` AND r.project = ?`
		args = append(args, normalizeProject(project))
	}
	if since != "" {
		where += ` AND m.timestamp >= ?`
		args = append(args, since)
	}
	if until != "" {
		where += ` AND m.timestamp <= ?`
		args = append(args, until)
	}

	q := fmt.Sprintf(`SELECT %s FROM messages m%s %s`, messageColumns, join, where)
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("fetch messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			continue
		}
		messages = append(messages, m)
	}

	// Sort by distance (closest first), then truncate to limit
	sortByDistance(messages, distances)
	if len(messages) > limit {
		messages = messages[:limit]
	}

	return messages, nil
}

// sortByDistance orders messages by their vector distance (ascending).
func sortByDistance(messages []Message, distances map[string]float64) {
	for i := 1; i < len(messages); i++ {
		for j := i; j > 0; j-- {
			di := distances[messages[j].ID]
			dj := distances[messages[j-1].ID]
			if di < dj {
				messages[j], messages[j-1] = messages[j-1], messages[j]
			} else {
				break
			}
		}
	}
}

// EmbedAsync generates and stores a vector embedding in the background.
// Non-fatal: logs warnings on failure, never blocks the caller.
func (s *Server) EmbedAsync(table, id, text string) {
	if s.Embedder == nil {
		return
	}
	go func() {
		vec, err := s.Embedder.Embed(context.Background(), text)
		if err != nil {
			s.Logger.Warn("embedding failed", "table", table, "id", id, "error", err)
			return
		}
		if err := s.StoreVector(table, id, vec); err != nil {
			s.Logger.Warn("store vector failed", "table", table, "id", id, "error", err)
		}
	}()
}

// BackfillEmbeddings embeds any messages and rooms that don't have vectors yet.
// Runs in the background on startup.
func (s *Server) BackfillEmbeddings(ctx context.Context) {
	if s.Embedder == nil {
		return
	}

	s.Logger.Info("Starting embedding backfill")

	// Backfill messages
	msgCount := 0
	rows, err := s.DB.Query(
		`SELECT m.id, m.content FROM messages m
		 LEFT JOIN message_vectors v ON m.id = v.message_id
		 WHERE v.message_id IS NULL`)
	if err != nil {
		s.Logger.Warn("backfill messages query failed", "error", err)
		return
	}

	var pending []struct{ id, content string }
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			continue
		}
		pending = append(pending, struct{ id, content string }{id, content})
	}
	rows.Close()

	for _, p := range pending {
		if ctx.Err() != nil {
			s.Logger.Info("Backfill cancelled", "messages_done", msgCount)
			return
		}
		vec, err := s.Embedder.Embed(ctx, p.content)
		if err != nil {
			s.Logger.Warn("backfill embed failed", "id", p.id, "error", err)
			continue
		}
		if err := s.StoreVector("message_vectors", p.id, vec); err != nil {
			s.Logger.Warn("backfill store failed", "id", p.id, "error", err)
			continue
		}
		msgCount++
	}

	// Backfill rooms
	roomCount := 0
	rows, err = s.DB.Query(
		`SELECT r.id, r.description, r.system_prompt FROM rooms r
		 LEFT JOIN room_vectors v ON r.id = v.room_id
		 WHERE v.room_id IS NULL`)
	if err != nil {
		s.Logger.Warn("backfill rooms query failed", "error", err)
		return
	}

	var roomPending []struct{ id, desc, prompt string }
	for rows.Next() {
		var id, desc, prompt string
		if err := rows.Scan(&id, &desc, &prompt); err != nil {
			continue
		}
		roomPending = append(roomPending, struct{ id, desc, prompt string }{id, desc, prompt})
	}
	rows.Close()

	for _, p := range roomPending {
		if ctx.Err() != nil {
			s.Logger.Info("Backfill cancelled", "rooms_done", roomCount)
			return
		}
		text := p.desc
		if p.prompt != "" {
			text += " " + p.prompt
		}
		vec, err := s.Embedder.Embed(ctx, text)
		if err != nil {
			s.Logger.Warn("backfill room embed failed", "id", p.id, "error", err)
			continue
		}
		if err := s.StoreVector("room_vectors", p.id, vec); err != nil {
			s.Logger.Warn("backfill room store failed", "id", p.id, "error", err)
			continue
		}
		roomCount++
	}

	s.Logger.Info("Embedding backfill complete", "messages", msgCount, "rooms", roomCount)
}
