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

	params := map[string]any{
		"message_ids": args.MessageIDs,
		"room_id":     args.RoomID,
		"last_n":      args.LastN,
		"after_id":    args.AfterID,
	}

	raw, warnings, err := r.clusterCall("get_messages", params)
	if err != nil {
		return msg(fmt.Sprintf("Error: cluster get_messages failed: %s", err.Error()))
	}

	var results []ClusterSearchResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster message results: %w", err)
	}

	if len(results) == 0 {
		var b strings.Builder
		b.WriteString("No messages found on any cluster node.")
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s) across cluster:\n\n", len(results))
	for _, m := range results {
		ts := m.Timestamp
		if len(ts) > 19 {
			ts = ts[:19]
		}
		fmt.Fprintf(&b, "---\n**#%s** [%s] [%s] %s in **%s** (%s):\n\n%s\n\n", m.ID, m.SourceNode, ts, m.Author, m.RoomID, m.MessageType, m.Content)
	}

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func (r *Registry) handleGetDigestCluster(args DigestInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.Since == "" {
		return msg("Error: since is required (ISO timestamp, e.g. 2026-03-31T12:00:00).")
	}

	params := map[string]any{
		"project": args.Project,
		"since":   args.Since,
	}

	raw, warnings, err := r.clusterCall("get_digest", params)
	if err != nil {
		return msg(fmt.Sprintf("Error: cluster get_digest failed: %s", err.Error()))
	}

	var results []ClusterDigestResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster digest results: %w", err)
	}

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
		return msg(fmt.Sprintf("Error: cluster read_notebook failed: %s", err.Error()))
	}

	var results []ClusterNotebookResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster notebook results: %w", err)
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

	types, _ := parseNotebookTypes(args.Types)

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
		writeNotebookEntry(&b, e, res.SourceNode)
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

	raw, warnings, err := r.clusterCall("list_rooms", params)
	if err != nil {
		return msg(fmt.Sprintf("Error: cluster read room failed: %s", err.Error()))
	}

	var results []ClusterRoomResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster room results: %w", err)
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
	fmt.Fprintf(&b, "[%s] **%s** [%s]\n", room.SourceNode, room.ID, room.Status)
	fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
	if room.Project != "" {
		fmt.Fprintf(&b, "**Project:** %s\n", room.Project)
	}
	if room.TechStack != "" {
		fmt.Fprintf(&b, "**Tech Stack:** %s\n", room.TechStack)
	}
	if room.Tags != "" {
		fmt.Fprintf(&b, "**Tags:** %s\n", room.Tags)
	}
	if room.SystemPrompt != "" {
		fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
	}
	if room.RelatedRooms != "" {
		fmt.Fprintf(&b, "**Related Rooms:** %s\n", room.RelatedRooms)
	}
	if room.Repo != "" {
		fmt.Fprintf(&b, "**Repo:** %s\n", room.Repo)
	}
	fmt.Fprintf(&b, "**Created:** %s\n", room.CreatedAt)
	fmt.Fprintf(&b, "**Updated:** %s\n", room.UpdatedAt)

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

// handleReadRoomClusterWithMessages serves read_room(cluster_wide, include_last_n)
// by fetching the room transcript from the owning node and appending the last N
// messages, mirroring the local read_room rendering.
func (r *Registry) handleReadRoomClusterWithMessages(args ReadRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	params := map[string]any{
		"room_id": args.RoomID,
	}

	raw, warnings, err := r.clusterCall("read_transcript", params)
	if err != nil {
		return msg(fmt.Sprintf("Error: cluster read room failed: %s", err.Error()))
	}

	var result *ClusterReadTranscriptResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster room results: %w", err)
	}

	if result == nil {
		var b strings.Builder
		fmt.Fprintf(&b, "Error: room '%s' not found on any cluster node.", args.RoomID)
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	room := result.Room
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] **%s** [%s]\n", room.SourceNode, room.ID, room.Status)
	fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
	if room.Project != "" {
		fmt.Fprintf(&b, "**Project:** %s\n", room.Project)
	}
	if room.TechStack != "" {
		fmt.Fprintf(&b, "**Tech Stack:** %s\n", room.TechStack)
	}
	if room.Tags != "" {
		fmt.Fprintf(&b, "**Tags:** %s\n", room.Tags)
	}
	if room.SystemPrompt != "" {
		fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
	}
	if room.RelatedRooms != "" {
		fmt.Fprintf(&b, "**Related Rooms:** %s\n", room.RelatedRooms)
	}
	if room.Repo != "" {
		fmt.Fprintf(&b, "**Repo:** %s\n", room.Repo)
	}
	fmt.Fprintf(&b, "**Created:** %s\n", room.CreatedAt)
	fmt.Fprintf(&b, "**Updated:** %s\n", room.UpdatedAt)

	// Append the last N messages (cap 50, matching the local read_room handler).
	lastN := 0
	_, _ = fmt.Sscanf(args.IncludeLastN, "%d", &lastN)
	if lastN > 50 {
		lastN = 50
	}
	if lastN > 0 {
		messages := result.Messages
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
				appendMessageBlock(&b, m.ID, ts, m.Author, m.MessageType, m.Content, room.Repo)
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

	raw, warnings, err := r.clusterCall("read_transcript", params)
	if err != nil {
		return msg(fmt.Sprintf("Error: cluster read_transcript failed: %s", err.Error()))
	}

	var result *ClusterReadTranscriptResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster read_transcript: %w", err)
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

func (r *Registry) handleSearchMessagesCluster(args SearchMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

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
		return msg(fmt.Sprintf("Error: cluster search failed: %s", err.Error()))
	}

	var results []ClusterSearchResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster search results: %w", err)
	}

	if len(results) == 0 {
		var b strings.Builder
		b.WriteString("No messages found matching the given filters (cluster-wide).")
		b.WriteString("\n\nNote: message bodies are node-local — each node only matches against its own messages, and remote nodes fall back to keyword (not semantic) matching. An empty cluster result is not proof that nothing matches; try a node-local search or different terms.")
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s) across cluster:\n\n", len(results))

	if args.SummaryOnly == "true" {
		for _, m := range results {
			ts := m.Timestamp
			if len(ts) > 16 {
				ts = ts[:16]
			}
			excerpt := m.Content
			if len(excerpt) > 120 {
				excerpt = excerpt[:120]
				if i := strings.LastIndex(excerpt, " "); i > 80 {
					excerpt = excerpt[:i]
				}
				excerpt += "..."
			}
			excerpt = strings.ReplaceAll(excerpt, "\n", " ")
			fmt.Fprintf(&b, "- [%s] #%.8s | %s | %s | %s | %s | %s\n", m.SourceNode, m.ID, ts, m.Author, m.RoomID, m.MessageType, excerpt)
		}
	} else {
		for _, m := range results {
			snippet := m.Content
			if args.FullContent != "true" && len(snippet) > 300 {
				snippet = snippet[:300] + "..."
			}
			fmt.Fprintf(&b, "- [%s] **#%s** [%s] %s in **%s** (%s):\n  %s\n\n", m.SourceNode, m.ID, m.Timestamp, m.Author, m.RoomID, m.MessageType, snippet)
		}
	}

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func (r *Registry) handleListRoomsCluster(args ListRoomsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	params := map[string]any{
		"project": args.Project,
		"tag":     args.Tag,
		"status":  args.Status,
		"search":  args.Search,
		"limit":   args.Limit,
		"offset":  args.Offset,
	}

	raw, warnings, err := r.clusterCall("list_rooms", params)
	if err != nil {
		return msg(fmt.Sprintf("Error: cluster list rooms failed: %s", err.Error()))
	}

	var results []ClusterRoomResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster room results: %w", err)
	}

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
			topic := rm.Description
			if len(topic) > 60 {
				topic = topic[:60] + "..."
			}
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

	params := map[string]any{
		"room_id":  args.RoomID,
		"room_ids": args.RoomIDs,
	}

	raw, warnings, err := r.clusterCall("room_stats", params)
	if err != nil {
		return msg(fmt.Sprintf("Error: cluster room stats failed: %s", err.Error()))
	}

	var stats *ClusterStatsResult
	if err := json.Unmarshal(raw, &stats); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster stats: %w", err)
	}

	if stats == nil {
		var b strings.Builder
		fmt.Fprintf(&b, "Error: room '%s' not found on any cluster node.", args.RoomID)
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	var b strings.Builder
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

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}
