package handlers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (r *Registry) handleGetMessagesCluster(args GetMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	var results []ClusterSearchResult
	if r.Server != nil {
		var local []council.Message
		var err error
		switch {
		case args.MessageIDs != "":
			ids := splitIDList(args.MessageIDs)
			ids, err = r.resolveIDList(ids)
			if err == nil {
				local, err = r.Server.GetMessagesByIDs(ids)
			}
		case args.RoomID != "" && args.AfterID != "":
			local, err = r.Server.GetMessagesAfterID(args.RoomID, args.AfterID)
		case args.RoomID != "":
			limit := 10
			if args.LastN != "" {
				if _, scanErr := fmt.Sscanf(args.LastN, "%d", &limit); scanErr != nil {
					limit = 10
				}
			}
			local, err = r.Server.GetRecentMessages(args.RoomID, limit)
		}
		if err == nil {
			for _, m := range local {
				results = append(results, clusterMessageFromLocal(m))
			}
		}
	}

	params := map[string]any{
		"message_ids": args.MessageIDs,
		"room_id":     args.RoomID,
		"last_n":      args.LastN,
		"after_id":    args.AfterID,
	}

	raw, warnings, err := r.clusterCall("get_messages", params)
	if err != nil {
		if len(results) > 0 {
			warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
			return r.renderClusterMessages(results, warnings)
		}
		return msg(fmt.Sprintf("Error: cluster get_messages failed: %s", err.Error()))
	}

	var remote []ClusterSearchResult
	if err := json.Unmarshal(raw, &remote); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster message results: %w", err)
	}
	results = append(results, remote...)
	results = dedupeClusterMessages(results)

	if len(results) == 0 {
		var b strings.Builder
		b.WriteString("No messages found on any cluster node.")
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	return r.renderClusterMessages(results, warnings)
}

func (r *Registry) renderClusterMessages(results []ClusterSearchResult, warnings []string) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s) across cluster:\n\n", len(results))
	for _, m := range results {
		ts := m.Timestamp
		if len(ts) > 19 {
			ts = ts[:19]
		}
		fmt.Fprintf(&b, "---\n**#%s** [%s] [%s] %s in **%s** (%s):\n\n%s\n\n", m.ID, m.SourceNode, ts, m.Author, m.RoomID, m.MessageType, council.DisplayContent(mapClusterMessage(m)))
	}

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func (r *Registry) handleGetDigestCluster(args DigestInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.Since == "" {
		return msg("Error: since is required (ISO timestamp, e.g. 2026-03-31T12:00:00).")
	}

	results := []ClusterDigestResult{}
	if r.Server != nil {
		local, err := r.Server.GetDigest(args.Project, args.Since)
		if err == nil {
			for _, d := range local {
				results = append(results, clusterDigestFromLocal(d))
			}
		}
	}

	params := map[string]any{
		"project": args.Project,
		"since":   args.Since,
	}

	raw, warnings, err := r.clusterCall("get_digest", params)
	if err != nil {
		if len(results) == 0 {
			return msg(fmt.Sprintf("Error: cluster get_digest failed: %s", err.Error()))
		}
		warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
	} else {
		var remote []ClusterDigestResult
		if err := json.Unmarshal(raw, &remote); err != nil {
			return nil, ToolOutput{}, fmt.Errorf("decode cluster digest results: %w", err)
		}
		results = append(results, remote...)
	}

	byRoom := map[string]ClusterDigestResult{}
	for _, d := range results {
		if existing, ok := byRoom[d.RoomID]; ok {
			if d.NewMessageCount > existing.NewMessageCount || existing.LatestMessageExcerpt == "" {
				byRoom[d.RoomID] = d
			}
		} else {
			byRoom[d.RoomID] = d
		}
	}
	results = results[:0]
	for _, d := range byRoom {
		results = append(results, d)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].NewMessageCount > results[j].NewMessageCount })

	outMap := map[string]any{
		"results":  results,
		"warnings": warnings,
	}

	out, err := json.MarshalIndent(outMap, "", "  ")
	if err != nil {
		return msg(fmt.Sprintf("Error formatting JSON: %s", err.Error()))
	}

	return msg(string(out))
}

func (r *Registry) handleReadNotebookCluster(args ReadNotebookInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	types, _ := parseNotebookTypes(args.Types)
	limit := 100
	if args.Limit != "" {
		if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
			limit = 100
		}
	}

	var results []ClusterNotebookResult
	if r.Server != nil {
		local, err := r.Server.GetNotebookEntries(args.Project, types, args.Since, args.Until, args.AfterID, limit)
		if err == nil {
			for _, e := range local {
				results = append(results, clusterNotebookFromLocal(e))
			}
		}
	}

	params := map[string]any{
		"project":  args.Project,
		"types":    args.Types,
		"since":    args.Since,
		"until":    args.Until,
		"after_id": args.AfterID,
		"limit":    args.Limit,
	}

	raw, warnings, err := r.clusterCall("read_notebook", params)
	if err != nil {
		if len(results) == 0 {
			return msg(fmt.Sprintf("Error: cluster read_notebook failed: %s", err.Error()))
		}
		warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
	} else {
		var remote []ClusterNotebookResult
		if err := json.Unmarshal(raw, &remote); err != nil {
			return nil, ToolOutput{}, fmt.Errorf("decode cluster notebook results: %w", err)
		}
		results = append(results, remote...)
	}

	byID := map[string]ClusterNotebookResult{}
	for _, e := range results {
		if existing, ok := byID[e.ID]; !ok || existing.SourceNode != localSourceNode() {
			byID[e.ID] = e
		}
	}
	results = results[:0]
	for _, e := range byID {
		results = append(results, e)
	}

	if len(results) == 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "No notebook entries for project '%s' on any cluster node.", args.Project)
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	// Phoenix merges and sorts by UUIDv7 ID (lexicographic == chronological,
	// valid across nodes); re-sort defensively in case of mixed peer versions.
	sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })
	if limit > 0 && len(results) > limit {
		results = results[len(results)-limit:]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Notebook — %s (cluster-wide)\n", args.Project)
	fmt.Fprintf(&b, "**Types:** %s | **Entries:** %d\n---\n", describeNotebookTypes(types), len(results))

	day := ""
	for _, res := range results {
		e := council.NotebookEntry{Message: mapClusterMessage(res.ClusterSearchResult), Repo: res.Repo}
		d := e.Timestamp.Format("2006-01-02")
		if d != day {
			day = d
			fmt.Fprintf(&b, "\n## %s\n", day)
		}
		writeNotebookEntry(&b, e, res.SourceNode, nil)
	}

	latest := results[len(results)-1].ID
	fmt.Fprintf(&b, "\n```json\n{\"latest_message_id\":\"%s\",\"entry_count\":%d}\n```\n", latest, len(results))

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func (r *Registry) handleReadRoomCluster(args ReadRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	// When include_last_n is requested, the list_rooms endpoint can't satisfy it
	// (it carries no messages). Route through read_transcript, which returns the
	// room plus its messages from the authoritative node, and append the tail.
	if args.IncludeLastN != "" {
		return r.handleReadRoomClusterWithMessages(args)
	}

	// We use the list_rooms cluster call with a search for the specific ID
	params := map[string]any{
		"search": args.RoomID,
	}

	var results []ClusterRoomResult
	if r.Server != nil {
		if room, err := r.Server.GetRoom(args.RoomID); err == nil {
			results = append(results, clusterRoomFromLocal(room))
		}
	}

	raw, warnings, err := r.clusterCall("list_rooms", params)
	if err != nil {
		if len(results) == 0 {
			return msg(fmt.Sprintf("Error: cluster read room failed: %s", err.Error()))
		}
		warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
	} else {
		var remote []ClusterRoomResult
		if err := json.Unmarshal(raw, &remote); err != nil {
			return nil, ToolOutput{}, fmt.Errorf("decode cluster room results: %w", err)
		}
		results = append(results, remote...)
	}

	// Pick the copy with the most recent UpdatedAt — the local node may hold a
	// stub with no topic/messages while the authoritative copy lives on a peer.
	var room *ClusterRoomResult
	var bestTime time.Time
	for i, res := range results {
		if res.ID == args.RoomID {
			t := parseClusterTime(res.UpdatedAt)
			if room == nil || t.After(bestTime) {
				room = &results[i]
				bestTime = t
			}
		}
	}

	if room == nil {
		var b strings.Builder
		fmt.Fprintf(&b, "Error: room '%s' not found on any cluster node.", args.RoomID)
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	var b strings.Builder
	writeClusterRoomHeader(&b, *room)

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

// writeClusterRoomHeader renders a cluster room's metadata block — shared by
// handleReadRoomCluster and handleReadRoomClusterWithMessages, which otherwise
// render the identical header byte-for-byte.
func writeClusterRoomHeader(b *strings.Builder, room ClusterRoomResult) {
	fmt.Fprintf(b, "[%s] **%s** [%s]\n", room.SourceNode, room.ID, room.Status)
	fmt.Fprintf(b, "**Topic:** %s\n", room.Description)
	if room.Project != "" {
		fmt.Fprintf(b, "**Project:** %s\n", room.Project)
	}
	if room.TechStack != "" {
		fmt.Fprintf(b, "**Tech Stack:** %s\n", room.TechStack)
	}
	if room.Tags != "" {
		fmt.Fprintf(b, "**Tags:** %s\n", room.Tags)
	}
	if room.SystemPrompt != "" {
		fmt.Fprintf(b, "**System Prompt:** %s\n", room.SystemPrompt)
	}
	if room.RelatedRooms != "" {
		fmt.Fprintf(b, "**Related Rooms:** %s\n", room.RelatedRooms)
	}
	if room.Repo != "" {
		fmt.Fprintf(b, "**Repo:** %s\n", room.Repo)
	}
	fmt.Fprintf(b, "**Created:** %s\n", room.CreatedAt)
	fmt.Fprintf(b, "**Updated:** %s\n", room.UpdatedAt)
}

// handleReadRoomClusterWithMessages serves read_room(cluster_wide, include_last_n)
// by fetching the room transcript from the owning node and appending the last N
// messages, mirroring the local read_room rendering.
func (r *Registry) handleReadRoomClusterWithMessages(args ReadRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	params := map[string]any{
		"room_id": args.RoomID,
	}

	localResult := r.localTranscriptClusterResult(args.RoomID)
	raw, warnings, err := r.clusterCall("read_transcript", params)
	if err != nil {
		if localResult == nil {
			return msg(fmt.Sprintf("Error: cluster read room failed: %s", err.Error()))
		}
		warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
	} else {
		var remote *ClusterReadTranscriptResult
		if err := json.Unmarshal(raw, &remote); err != nil {
			return nil, ToolOutput{}, fmt.Errorf("decode cluster room results: %w", err)
		}
		if remote != nil && (localResult == nil || len(remote.Messages) > len(localResult.Messages)) {
			localResult = remote
		}
	}

	if localResult == nil {
		var b strings.Builder
		fmt.Fprintf(&b, "Error: room '%s' not found on any cluster node.", args.RoomID)
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	room := localResult.Room
	var b strings.Builder
	writeClusterRoomHeader(&b, room)

	// Append the last N messages (cap 50, matching the local read_room handler).
	lastN := 0
	_, _ = fmt.Sscanf(args.IncludeLastN, "%d", &lastN)
	if lastN > 50 {
		lastN = 50
	}
	if lastN > 0 {
		messages := localResult.Messages
		if len(messages) > lastN {
			messages = messages[len(messages)-lastN:]
		}
		if len(messages) > 0 {
			fmt.Fprintf(&b, "\n---\n**Recent messages (%d):**\n", len(messages))
			for _, m := range messages {
				ts := m.Timestamp
				if len(ts) > 19 {
					ts = ts[:19]
				}
				appendMessageBlock(&b, m.ID, ts, m.Author, m.MessageType, council.DisplayContent(mapClusterMessage(m)), room.Repo)
			}
		}
	}

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func (r *Registry) handleReadTranscriptCluster(args ReadTranscriptInput, roomID string) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	params := map[string]any{
		"room_id": roomID,
	}

	result := r.localTranscriptClusterResult(roomID)
	raw, warnings, err := r.clusterCall("read_transcript", params)
	if err != nil {
		if result == nil {
			return msg(fmt.Sprintf("Error: cluster read_transcript failed: %s", err.Error()))
		}
		warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
	} else {
		var remote *ClusterReadTranscriptResult
		if err := json.Unmarshal(raw, &remote); err != nil {
			return nil, ToolOutput{}, fmt.Errorf("decode cluster read_transcript: %w", err)
		}
		if remote != nil && (result == nil || len(remote.Messages) > len(result.Messages)) {
			result = remote
		}
	}

	if result == nil {
		var b strings.Builder
		fmt.Fprintf(&b, "Error: room '%s' not found on any cluster node.", roomID)
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	room := mapClusterRoom(result.Room)
	var messages []council.Message

	// Filter down the cluster messages just like Go does
	limit := 0
	if args.LastN != "" {
		_, _ = fmt.Sscanf(args.LastN, "%d", &limit)
	}
	afterID := ""
	if args.AfterID != "" {
		afterID = args.AfterID
	}

	var filtered []council.Message
	for _, m := range result.Messages {
		if afterID != "" && m.ID <= afterID {
			continue
		}

		if args.Mode == "changelog" {
			if m.MessageType != "decision" && m.MessageType != "action" && m.MessageType != "summary" {
				continue
			}
		}

		filtered = append(filtered, mapClusterMessage(m))
	}

	if args.Mode == "summary" {
		var summary []council.Message
		seen := make(map[string]bool)
		// Go backwards to get latest per type
		for i := len(filtered) - 1; i >= 0; i-- {
			m := filtered[i]
			if !seen[m.MessageType] {
				seen[m.MessageType] = true
				summary = append([]council.Message{m}, summary...) // prepend
			}
		}
		messages = summary
	} else if limit > 0 && len(filtered) > limit {
		messages = filtered[len(filtered)-limit:]
	} else {
		messages = filtered
	}

	if result.Pinned != nil && afterID != "" {
		// Include pinned for context if doing afterID delta read
		pinnedMsg := mapClusterMessage(*result.Pinned)
		messages = append([]council.Message{pinnedMsg}, messages...)
	}

	transcript := council.FormatTranscript(room, messages)

	var b strings.Builder
	b.WriteString(transcript)
	if len(warnings) > 0 {
		b.WriteString("\n\n---\n")
		for _, w := range warnings {
			fmt.Fprintf(&b, "**Cluster Warning:** %s\n", w)
		}
	}

	return msg(b.String())
}

func (r *Registry) localTranscriptClusterResult(roomID string) *ClusterReadTranscriptResult {
	if r.Server == nil {
		return nil
	}
	room, err := r.Server.GetRoom(roomID)
	if err != nil {
		return nil
	}
	messages, err := r.Server.GetTranscript(roomID)
	if err != nil {
		return nil
	}
	result := &ClusterReadTranscriptResult{
		Room:     clusterRoomFromLocal(room),
		Messages: make([]ClusterSearchResult, 0, len(messages)),
	}
	for _, m := range messages {
		cm := clusterMessageFromLocal(m)
		result.Messages = append(result.Messages, cm)
		if m.Pinned {
			pinned := cm
			result.Pinned = &pinned
		}
	}
	return result
}

func (r *Registry) handleSearchMessagesCluster(args SearchMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	var results []ClusterSearchResult
	if r.Server != nil {
		effectiveRoomIDs := args.RoomIDs
		if args.RoomID != "" && args.RoomIDs == "" {
			effectiveRoomIDs = args.RoomID
		}
		limit := 20
		if args.Limit != "" {
			if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
				limit = 20
			}
		}
		local, err := r.Server.SearchMessages(args.Query, args.Author, args.MessageType, effectiveRoomIDs, args.Project, args.Since, args.Until, limit)
		if err == nil {
			for _, m := range local {
				results = append(results, clusterMessageFromLocal(m))
			}
		}
	}

	params := map[string]any{
		"query":        args.Query,
		"author":       args.Author,
		"message_type": args.MessageType,
		"room_id":      args.RoomID,
		"room_ids":     args.RoomIDs,
		"project":      args.Project,
		"since":        args.Since,
		"until":        args.Until,
		"limit":        args.Limit,
		"semantic":     args.Semantic,
	}

	raw, warnings, err := r.clusterCall("search_messages", params)
	if err != nil {
		if len(results) > 0 {
			warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
			return r.renderClusterSearch(args, results, warnings)
		}
		return msg(fmt.Sprintf("Error: cluster search failed: %s", err.Error()))
	}

	var remote []ClusterSearchResult
	if err := json.Unmarshal(raw, &remote); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster search results: %w", err)
	}
	results = append(results, remote...)
	results = dedupeClusterMessages(results)
	sort.Slice(results, func(i, j int) bool { return results[i].ID > results[j].ID })

	if len(results) == 0 {
		var b strings.Builder
		b.WriteString("No messages found matching the given filters (cluster-wide).")
		b.WriteString("\n\nNote: message bodies are node-local — each node only matches against its own messages, and remote nodes fall back to keyword (not semantic) matching. An empty cluster result is not proof that nothing matches; try a node-local search or different terms.")
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	return r.renderClusterSearch(args, results, warnings)
}

func dedupeClusterMessages(results []ClusterSearchResult) []ClusterSearchResult {
	byID := map[string]ClusterSearchResult{}
	for _, m := range results {
		if existing, ok := byID[m.ID]; !ok || existing.SourceNode != localSourceNode() {
			byID[m.ID] = m
		}
	}
	out := make([]ClusterSearchResult, 0, len(byID))
	for _, m := range byID {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) renderClusterSearch(args SearchMessagesInput, results []ClusterSearchResult, warnings []string) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s) across cluster:\n\n", len(results))

	if args.SummaryOnly == "true" {
		for _, m := range results {
			ts := m.Timestamp
			if len(ts) > 16 {
				ts = ts[:16]
			}
			excerpt := council.TruncateRunes(council.DisplayContent(mapClusterMessage(m)), 120, " ", 80)
			excerpt = strings.ReplaceAll(excerpt, "\n", " ")
			fmt.Fprintf(&b, "- [%s] #%s | %s | %s | %s | %s | %s\n", m.SourceNode, m.ID, ts, m.Author, m.RoomID, m.MessageType, excerpt)
		}
	} else {
		for _, m := range results {
			snippet := council.DisplayContent(mapClusterMessage(m))
			if args.FullContent != "true" {
				snippet = council.TruncateRunes(snippet, 300, "", 0)
			}
			fmt.Fprintf(&b, "- [%s] **#%s** [%s] %s in **%s** (%s):\n  %s\n\n", m.SourceNode, m.ID, m.Timestamp, m.Author, m.RoomID, m.MessageType, snippet)
		}
	}

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func (r *Registry) handleListRoomsCluster(args ListRoomsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	var results []ClusterRoomResult
	if r.Server != nil {
		limit := 50
		if args.Limit != "" {
			if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
				limit = 50
			}
		}
		offset := 0
		if args.Offset != "" {
			if _, err := fmt.Sscanf(args.Offset, "%d", &offset); err != nil {
				offset = 0
			}
		}
		var projectNotIn []string
		if args.ProjectNotIn != "" {
			for _, p := range strings.Split(args.ProjectNotIn, ",") {
				if p = strings.TrimSpace(p); p != "" {
					projectNotIn = append(projectNotIn, p)
				}
			}
		}
		localRooms, err := r.Server.ListRoomsFiltered(council.ListRoomsOptions{
			Project:      args.Project,
			ProjectNotIn: projectNotIn,
			Tag:          args.Tag,
			Status:       args.Status,
			Search:       args.Search,
			RelatedTo:    args.RelatedTo,
			Limit:        limit,
			Offset:       offset,
		})
		if err == nil {
			for _, rm := range localRooms {
				results = append(results, clusterRoomFromLocal(rm))
			}
		}
	}

	params := map[string]any{
		"project":        args.Project,
		"project_not_in": args.ProjectNotIn,
		"tag":            args.Tag,
		"status":         args.Status,
		"search":         args.Search,
		"related_to":     args.RelatedTo,
		"limit":          args.Limit,
		"offset":         args.Offset,
	}

	raw, warnings, err := r.clusterCall("list_rooms", params)
	if err != nil {
		if len(results) > 0 {
			warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
			return r.renderClusterRooms(args, results, warnings)
		}
		return msg(fmt.Sprintf("Error: cluster list rooms failed: %s", err.Error()))
	}

	var remote []ClusterRoomResult
	if err := json.Unmarshal(raw, &remote); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster room results: %w", err)
	}
	results = append(results, remote...)

	byID := map[string]ClusterRoomResult{}
	for _, rm := range results {
		if existing, ok := byID[rm.ID]; !ok || parseClusterTime(rm.UpdatedAt).After(parseClusterTime(existing.UpdatedAt)) {
			byID[rm.ID] = rm
		}
	}
	results = results[:0]
	for _, rm := range byID {
		results = append(results, rm)
	}
	sort.Slice(results, func(i, j int) bool {
		return parseClusterTime(results[i].UpdatedAt).After(parseClusterTime(results[j].UpdatedAt))
	})

	return r.renderClusterRooms(args, results, warnings)
}

func (r *Registry) renderClusterRooms(args ListRoomsInput, results []ClusterRoomResult, warnings []string) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if len(results) == 0 {
		var b strings.Builder
		b.WriteString("No rooms found matching the given filters (cluster-wide).")
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d room(s) across cluster:\n\n", len(results))

	useVerbose := args.Verbose == "true" || args.Compact == "false"
	if !useVerbose {
		for _, rm := range results {
			topic := council.TruncateRunes(rm.Description, 60, "", 0)
			project := rm.Project
			if project == "" {
				project = "-"
			}
			updatedAt := rm.UpdatedAt
			if len(updatedAt) > 16 {
				updatedAt = updatedAt[:16]
			}
			fmt.Fprintf(&b, "- [%s] **%s** | %s | %s | %s | %s\n", rm.SourceNode, rm.ID, project, rm.Status, topic, updatedAt)
		}
	} else {
		for _, rm := range results {
			fmt.Fprintf(&b, "- [%s] **%s** [%s]", rm.SourceNode, rm.ID, rm.Status)
			if rm.Project != "" {
				fmt.Fprintf(&b, " | project: %s", rm.Project)
			}
			if rm.Tags != "" {
				fmt.Fprintf(&b, " | tags: %s", rm.Tags)
			}
			fmt.Fprintf(&b, "\n  %s\n", rm.Description)
			if rm.TechStack != "" {
				fmt.Fprintf(&b, "  Tech: %s\n", rm.TechStack)
			}
			if rm.RelatedRooms != "" {
				fmt.Fprintf(&b, "  Related: %s\n", rm.RelatedRooms)
			}
			fmt.Fprintf(&b, "  Last activity: %s\n", rm.UpdatedAt)
		}
	}

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func (r *Registry) handleRoomStatsCluster(args RoomStatsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	var localStats []ClusterStatsResult
	if r.Server != nil {
		ids := splitRoomIDArgs(args.RoomID, args.RoomIDs)
		for _, id := range ids {
			stats, err := r.Server.GetRoomStats(id)
			if err == nil {
				localStats = append(localStats, clusterStatsFromLocal(stats))
			}
		}
	}

	params := map[string]any{
		"room_id":  args.RoomID,
		"room_ids": args.RoomIDs,
	}

	raw, warnings, err := r.clusterCall("room_stats", params)
	if err != nil {
		if len(localStats) > 0 {
			warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
			return r.renderClusterStats(localStats, warnings)
		}
		return msg(fmt.Sprintf("Error: cluster room stats failed: %s", err.Error()))
	}

	var stats *ClusterStatsResult
	if err := json.Unmarshal(raw, &stats); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster stats: %w", err)
	}

	results := localStats
	if stats != nil {
		results = append(results, *stats)
	}
	if len(results) == 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "Error: room '%s' not found on any cluster node.", args.RoomID)
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}
	return r.renderClusterStats(bestClusterStats(results), warnings)
}

func splitRoomIDArgs(roomID, roomIDs string) []string {
	seen := map[string]bool{}
	var ids []string
	if roomIDs != "" {
		for _, id := range strings.Split(roomIDs, ",") {
			id = strings.TrimSpace(id)
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if roomID != "" && !seen[roomID] {
		ids = append(ids, roomID)
	}
	return ids
}

func bestClusterStats(results []ClusterStatsResult) []ClusterStatsResult {
	byID := map[string]ClusterStatsResult{}
	for _, stats := range results {
		if existing, ok := byID[stats.RoomID]; !ok || stats.MessageCount > existing.MessageCount {
			byID[stats.RoomID] = stats
		}
	}
	out := make([]ClusterStatsResult, 0, len(byID))
	for _, stats := range byID {
		out = append(out, stats)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RoomID < out[j].RoomID })
	return out
}

func (r *Registry) renderClusterStats(results []ClusterStatsResult, warnings []string) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult
	var b strings.Builder
	for i, stats := range results {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		fmt.Fprintf(&b, "[%s] **%s** [%s]\n", stats.SourceNode, stats.RoomID, stats.Status)
		fmt.Fprintf(&b, "**Messages:** %d\n", stats.MessageCount)
		if stats.LatestMessageID != "" {
			fmt.Fprintf(&b, "**Latest message ID:** %.8s\n", stats.LatestMessageID)
		}

		if len(stats.Participants) > 0 {
			var parts []string
			for author, count := range stats.Participants {
				parts = append(parts, fmt.Sprintf("%s (%d)", author, count))
			}
			fmt.Fprintf(&b, "**Participants:** %s\n", strings.Join(parts, ", "))
			if stats.FirstMessage != "" {
				fmt.Fprintf(&b, "**First message:** %s\n", stats.FirstMessage)
			}
			if stats.LastMessage != "" {
				fmt.Fprintf(&b, "**Last message:** %s\n", stats.LastMessage)
			}
		}

		if len(stats.TypeCounts) > 0 {
			var types []string
			for msgType, count := range stats.TypeCounts {
				types = append(types, fmt.Sprintf("%s: %d", msgType, count))
			}
			fmt.Fprintf(&b, "**Types:** %s\n", strings.Join(types, ", "))
		}
	}

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}
