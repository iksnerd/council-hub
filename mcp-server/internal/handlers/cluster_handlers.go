package handlers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
			return r.renderClusterMessages(results, warnings, nil)
		}
		return msg(fmt.Sprintf("Error: cluster get_messages failed: %s", err.Error()))
	}

	var remote []ClusterSearchResult
	if err := json.Unmarshal(raw, &remote); err != nil {
		return nil, ToolOutput{}, fmt.Errorf("decode cluster message results: %w", err)
	}
	results = append(results, remote...)
	results = dedupeClusterMessages(results)

	var missing []string
	if args.MessageIDs != "" {
		missing = missingClusterIDs(splitIDList(args.MessageIDs), results)
	}

	if len(results) == 0 {
		var b strings.Builder
		b.WriteString("No messages found on any cluster node.")
		if len(missing) > 0 {
			fmt.Fprintf(&b, " Not found: %s.", strings.Join(missing, ", "))
		}
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	return r.renderClusterMessages(results, warnings, missing)
}

// missingClusterIDs returns the requested ids no node returned. A requested id
// may be a prefix (or room#prefix), so it counts as found when a result's id
// starts with it.
func missingClusterIDs(requested []string, results []ClusterSearchResult) []string {
	var missing []string
	for _, id := range requested {
		want := id
		if i := strings.LastIndex(want, "#"); i >= 0 {
			want = want[i+1:]
		}
		found := false
		for _, m := range results {
			if want != "" && strings.HasPrefix(m.ID, want) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, id)
		}
	}
	return missing
}

func (r *Registry) renderClusterMessages(results []ClusterSearchResult, warnings []string, missing []string) (*mcp.CallToolResult, ToolOutput, error) {
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

	if len(missing) > 0 {
		fmt.Fprintf(&b, "Not found: %s.\n", strings.Join(missing, ", "))
	}
	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func (r *Registry) handleGetDigestCluster(args DigestInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.Since == "" {
		return msg("Error: since is required (ISO timestamp, e.g. 2026-03-31T12:00:00).")
	}

	// Local rows come from the full local query and are authoritative for rooms
	// this node owns; peers' rows (Phoenix also echoes this node's own rooms
	// back, tagged with its Erlang name) only fill in rooms not seen locally.
	var rooms []council.DigestEntry
	seen := map[string]bool{}
	if r.Server != nil {
		local, err := r.Server.GetDigest(args.Project, args.Since)
		if err == nil {
			for _, d := range local {
				d.LatestExcerpt = digestExcerpt(d.LatestExcerpt)
				d.SourceNode = localSourceNode()
				seen[d.RoomID] = true
				rooms = append(rooms, d)
			}
		}
	}

	params := map[string]any{
		"project": args.Project,
		"since":   args.Since,
	}

	raw, warnings, err := r.clusterCall("get_digest", params)
	if err != nil {
		if len(rooms) == 0 {
			return msg(fmt.Sprintf("Error: cluster get_digest failed: %s", err.Error()))
		}
		warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
	} else {
		var remote []ClusterDigestResult
		if err := json.Unmarshal(raw, &remote); err != nil {
			return nil, ToolOutput{}, fmt.Errorf("decode cluster digest results: %w", err)
		}
		legacy := 0
		noCounts := 0
		for _, d := range remote {
			if seen[d.RoomID] {
				continue
			}
			seen[d.RoomID] = true
			if d.DecisionCount == nil || d.SynthesisCount == nil {
				noCounts++
			}
			if d.LatestMessageID == "" {
				legacy++
			}
			entry := council.DigestEntry{
				RoomID:          d.RoomID,
				NewMessages:     d.NewMessageCount,
				LatestAuthor:    d.LatestAuthor,
				LatestExcerpt:   d.LatestMessageExcerpt,
				Tags:            d.Tags,
				LatestMessageID: d.LatestMessageID,
				PostedFrom:      d.PostedFrom,
				SourceNode:      d.SourceNode,
			}
			if d.DecisionCount != nil {
				entry.DecisionCount = *d.DecisionCount
			}
			if d.SynthesisCount != nil {
				entry.SynthesisCount = *d.SynthesisCount
			}
			rooms = append(rooms, entry)
		}
		if legacy > 0 {
			warnings = append(warnings, fmt.Sprintf("%d peer room(s) come from a node that does not report latest_message_id/latest_author/tags (older version); those fields are empty", legacy))
		}
		if noCounts > 0 {
			warnings = append(warnings, fmt.Sprintf("%d peer room(s) come from a node that does not report decision_count or synthesis_count (older version); those fields are empty", noCounts))
		}
	}

	// Same ordering as the local digest, which is newest activity first. Rows
	// without a latest_message_id (legacy peers) sort last, by unread count.
	sort.SliceStable(rooms, func(i, j int) bool {
		a, b := rooms[i], rooms[j]
		if a.LatestMessageID != b.LatestMessageID {
			return a.LatestMessageID > b.LatestMessageID
		}
		return a.NewMessages > b.NewMessages
	})

	if warnings == nil {
		warnings = []string{}
	}
	out, err := renderDigest(rooms, args, "", warnings)
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
		e := council.NotebookEntry{Message: mapClusterMessage(res.ClusterSearchResult), Repo: res.Repo, WorkItems: res.WorkItems}
		d := e.Timestamp.Format("2006-01-02")
		if d != day {
			day = d
			fmt.Fprintf(&b, "\n## %s\n", day)
		}
		writeNotebookEntry(&b, nil, e, res.SourceNode, nil)
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
	// The local read_room also folds in the pinned message and latest-per-type;
	// a peer's room is header-only here, so say how to get the rest.
	b.WriteString("\n(Room metadata only. Pass include_last_n=N for recent messages, or read_transcript(room_id, cluster_wide=true) for the summary view.)\n")

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
	if room.WorkItems != "" {
		fmt.Fprintf(b, "**Work items:** %s\n", room.WorkItems)
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
			remoteIDList := make([]string, 0, len(messages))
			for _, m := range messages {
				remoteIDList = append(remoteIDList, m.ID)
			}
			remoteIDs := council.NewShortIDs(remoteIDList)
			for _, m := range messages {
				ts := m.Timestamp
				if len(ts) > 19 {
					ts = ts[:19]
				}
				appendMessageBlock(&b, remoteIDs, m.ID, ts, m.Author, m.MessageType, council.DisplayContent(mapClusterMessage(m)), room.Repo, room.WorkItems)
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
		local, err := r.Server.SearchMessagesByWorkItem(args.WorkItem, args.Query, args.Author, args.MessageType, effectiveRoomIDs, args.Project, args.Since, args.Until, limit)
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
		"work_item":    args.WorkItem,
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
	// Every node returned up to limit rows; keep the newest limit overall, as
	// the local search would.
	searchLimit := 20
	if args.Limit != "" {
		if _, err := fmt.Sscanf(args.Limit, "%d", &searchLimit); err != nil {
			searchLimit = 20
		}
	}
	if searchLimit > 0 && len(results) > searchLimit {
		results = results[:searchLimit]
	}

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

func (r *Registry) handleListRoomsCluster(args ListRoomsInput) (*mcp.CallToolResult, ListRoomsOutput, error) {
	// Same defaults as the local list_rooms: limit 50, and resolved rooms hidden
	// unless a status, search or related_to is given (status=all lifts it).
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
	status := args.Status
	hideResolved := false
	switch {
	case status == "all":
		status = ""
	case status == "" && args.Search == "" && args.RelatedTo == "":
		hideResolved = true
	}
	// Each node returns its first limit+offset rooms; the page is cut after the
	// merge, since a per-node offset would skip the wrong rows.
	window := limit + offset

	var results []ClusterRoomResult
	var localResolvedOpts council.ListRoomsOptions
	if r.Server != nil {
		var projectNotIn []string
		if args.ProjectNotIn != "" {
			for _, p := range strings.Split(args.ProjectNotIn, ",") {
				if p = strings.TrimSpace(p); p != "" {
					projectNotIn = append(projectNotIn, p)
				}
			}
		}
		opts := council.ListRoomsOptions{
			Project:      args.Project,
			ProjectNotIn: projectNotIn,
			Tag:          args.Tag,
			Status:       status,
			Search:       args.Search,
			RelatedTo:    args.RelatedTo,
			Limit:        window,
		}
		if hideResolved {
			opts.ExcludeStatus = "resolved"
			localResolvedOpts = opts
			localResolvedOpts.ExcludeStatus, localResolvedOpts.Status = "", "resolved"
		}
		localRooms, err := r.Server.ListRoomsFiltered(opts)
		if err == nil {
			ids := make([]string, len(localRooms))
			for i, rm := range localRooms {
				ids[i] = rm.ID
			}
			counts := r.Server.GetMessageCounts()
			pins := r.Server.GetPinnedExcerpts(ids)
			for _, rm := range localRooms {
				cr := clusterRoomFromLocal(rm)
				n := counts[rm.ID]
				cr.MessageCount = &n
				cr.PinnedExcerpt = pins[rm.ID]
				results = append(results, cr)
			}
		}
	}

	params := map[string]any{
		"project":        args.Project,
		"project_not_in": args.ProjectNotIn,
		"tag":            args.Tag,
		"status":         status,
		"search":         args.Search,
		"related_to":     args.RelatedTo,
		"limit":          strconv.Itoa(window),
		"offset":         "0",
	}

	raw, warnings, err := r.clusterCall("list_rooms", params)
	if err != nil {
		if len(results) > 0 {
			warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
			omitted := 0
			if hideResolved && r.Server != nil {
				if n, cerr := r.Server.CountRooms(localResolvedOpts); cerr == nil {
					omitted = n
				}
			}
			return r.renderClusterRooms(args, results, warnings, omitted)
		}
		return listRoomsResult(fmt.Sprintf("Error: cluster list rooms failed: %s", err.Error()), newListRoomsOutput(nil, nil))
	}

	var remote []ClusterRoomResult
	if err := json.Unmarshal(raw, &remote); err != nil {
		return nil, ListRoomsOutput{}, fmt.Errorf("decode cluster room results: %w", err)
	}
	for _, rm := range remote {
		if hideResolved && rm.Status == "resolved" {
			continue
		}
		results = append(results, rm)
	}

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

	if offset > len(results) {
		offset = len(results)
	}
	results = results[offset:]
	if len(results) > limit {
		results = results[:limit]
	}

	omitted := 0
	if hideResolved {
		var counted bool
		omitted, counted = r.countClusterResolved(params)
		if !counted {
			warnings = append(warnings, "resolved-room count covers this node only (peer count unavailable)")
			if r.Server != nil {
				if n, cerr := r.Server.CountRooms(localResolvedOpts); cerr == nil {
					omitted = n
				}
			}
		}
	}
	return r.renderClusterRooms(args, results, warnings, omitted)
}

// countClusterResolved asks the cluster for the ids of resolved rooms matching
// the same filters as the listing, and counts the distinct ids (Phoenix fans
// out to its own node too, so a room seen twice counts once). The bool is false
// when the call failed.
func (r *Registry) countClusterResolved(listParams map[string]any) (int, bool) {
	params := map[string]any{}
	for k, v := range listParams {
		params[k] = v
	}
	params["status"] = "resolved"
	delete(params, "limit")
	delete(params, "offset")
	raw, _, err := r.clusterCall("room_refs", params)
	if err != nil {
		return 0, false
	}
	var refs []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &refs); err != nil {
		return 0, false
	}
	ids := map[string]bool{}
	for _, ref := range refs {
		if ref.Status == "resolved" {
			ids[ref.ID] = true
		}
	}
	return len(ids), true
}

func (r *Registry) renderClusterRooms(args ListRoomsInput, results []ClusterRoomResult, warnings []string, omittedResolved int) (*mcp.CallToolResult, ListRoomsOutput, error) {
	hiddenNote := ""
	if omittedResolved > 0 {
		hiddenNote = fmt.Sprintf("\n(%d resolved room(s) not shown — pass status=resolved or status=all to include them.)\n", omittedResolved)
	}
	if len(results) == 0 {
		var b strings.Builder
		b.WriteString("No rooms found matching the given filters (cluster-wide).")
		b.WriteString(hiddenNote)
		formatClusterWarnings(&b, warnings)
		out := newListRoomsOutput(nil, warnings)
		out.OmittedResolved = omittedResolved
		return listRoomsResult(b.String(), out)
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
			// Message count and pinned excerpt come from the owning node; the read
			// cursor is node-local, so it is not shown here.
			counts := ""
			if rm.MessageCount != nil {
				counts = fmt.Sprintf(" | %d msgs", *rm.MessageCount)
			}
			if pin := listedPin(rm); pin != "" {
				counts += " | 📌 " + pin
			}
			fmt.Fprintf(&b, "- [%s] **%s** | %s | %s%s | %s | %s\n", rm.SourceNode, rm.ID, project, rm.Status, counts, topic, updatedAt)
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
			if pin := listedPin(rm); pin != "" {
				fmt.Fprintf(&b, "  📌 %s\n", pin)
			}
			if rm.TechStack != "" {
				fmt.Fprintf(&b, "  Tech: %s\n", rm.TechStack)
			}
			if rm.RelatedRooms != "" {
				fmt.Fprintf(&b, "  Related: %s\n", rm.RelatedRooms)
			}
			fmt.Fprintf(&b, "  Last activity: %s\n", rm.UpdatedAt)
		}
	}

	b.WriteString(hiddenNote)
	formatClusterWarnings(&b, warnings)
	listed := make([]ListedRoom, 0, len(results))
	for _, rm := range results {
		listed = append(listed, listedRoomFromCluster(rm))
	}
	out := newListRoomsOutput(listed, warnings)
	out.OmittedResolved = omittedResolved
	return listRoomsResult(b.String(), out)
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

// listedPin is a peer's pinned excerpt flattened to one line and clipped to the
// same width as the local listing.
func listedPin(rm ClusterRoomResult) string {
	return council.TruncateRunes(strings.ReplaceAll(rm.PinnedExcerpt, "\n", " "), 60, " ", 40)
}
