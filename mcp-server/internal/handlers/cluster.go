package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ClusterSearchResult represents a message from a cluster-wide search.
type ClusterSearchResult struct {
	ID          string `json:"id"`
	RoomID      string `json:"room_id"`
	Author      string `json:"author"`
	Content     string `json:"content"`
	MessageType string `json:"message_type"`
	IsSummary   bool   `json:"is_summary"`
	ReplyTo     string `json:"reply_to"`
	Pinned      bool   `json:"pinned"`
	Timestamp   string `json:"timestamp"`
	SourceNode  string `json:"source_node"`
}

// ClusterRoomResult represents a room from a cluster-wide listing.
type ClusterRoomResult struct {
	ID           string `json:"id"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Project      string `json:"project"`
	TechStack    string `json:"tech_stack"`
	Tags         string `json:"tags"`
	SystemPrompt string `json:"system_prompt"`
	RelatedRooms string `json:"related_rooms"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	SourceNode   string `json:"source_node"`
}

// ClusterStatsResult represents room stats from a cluster-wide query.
type ClusterStatsResult struct {
	RoomID          string         `json:"room_id"`
	Status          string         `json:"status"`
	MessageCount    int            `json:"message_count"`
	Participants    map[string]int `json:"participants"`
	TypeCounts      map[string]int `json:"type_counts"`
	FirstMessage    string         `json:"first_message"`
	LastMessage     string         `json:"last_message"`
	LatestMessageID string         `json:"latest_message_id"`
	SourceNode      string         `json:"source_node"`
}

// ClusterReadTranscriptResult represents raw data to format a transcript.
type ClusterReadTranscriptResult struct {
	Room     ClusterRoomResult     `json:"room"`
	Messages []ClusterSearchResult `json:"messages"`
	Pinned   *ClusterSearchResult  `json:"pinned"`
}

// clusterResponse is the generic response from Phoenix cluster API.
type clusterResponse[T any] struct {
	Results  T        `json:"results"`
	Warnings []string `json:"warnings"`
}

// clusterCall makes an HTTP POST to the Phoenix internal cluster API.
func (r *Registry) clusterCall(endpoint string, params map[string]any) (json.RawMessage, []string, error) {
	if r.HTTPClient == nil || r.PhoenixURL == "" {
		return nil, nil, fmt.Errorf("cluster queries not configured (no Phoenix URL)")
	}

	body, err := json.Marshal(params)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal params: %w", err)
	}

	url := r.PhoenixURL + "/api/internal/cluster/" + endpoint
	resp, err := r.HTTPClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("cluster call to %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("cluster %s returned %d: %s", endpoint, resp.StatusCode, string(msg))
	}

	var raw struct {
		Results  json.RawMessage `json:"results"`
		Warnings []string        `json:"warnings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("decode cluster response: %w", err)
	}

	return raw.Results, raw.Warnings, nil
}

type ClusterDigestResult struct {
	RoomID               string `json:"room_id"`
	NewMessageCount      int    `json:"new_message_count"`
	LatestMessageExcerpt string `json:"latest_message_excerpt"`
	SourceNode           string `json:"source_node"`
}

func (r *Registry) handleGetMessagesCluster(args GetMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	params := map[string]any{
		"message_ids": args.MessageIDs,
		"room_id":     args.RoomID,
		"last_n":      args.LastN,
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
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

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

	if len(results) == 0 {
		var b strings.Builder
		projectNote := ""
		if args.Project != "" {
			projectNote = fmt.Sprintf(" in project '%s'", args.Project)
		}
		fmt.Fprintf(&b, "No new activity%s since %s across cluster.", projectNote, args.Since)
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	var b strings.Builder
	projectNote := ""
	if args.Project != "" {
		projectNote = fmt.Sprintf(" [%s]", args.Project)
	}
	fmt.Fprintf(&b, "# Cluster Activity Digest%s \u2014 since %s\n\n", projectNote, args.Since)
	fmt.Fprintf(&b, "%d room(s) with new activity:\n\n", len(results))

	for _, d := range results {
		excerpt := d.LatestMessageExcerpt
		if len(excerpt) > 120 {
			excerpt = excerpt[:120] + "..."
		}
		excerpt = strings.ReplaceAll(excerpt, "\n", " ")
		fmt.Fprintf(&b, "- [%s] **%s** | %d new msg(s) | %s\n", d.SourceNode, d.RoomID, d.NewMessageCount, excerpt)
	}

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func formatClusterWarnings(b *strings.Builder, warnings []string) {
	if len(warnings) > 0 {
		b.WriteString("\n---\n")
		for _, w := range warnings {
			fmt.Fprintf(b, "**Warning:** %s\n", w)
		}
	}
}

func parseClusterTime(ts string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05", ts)
	return t
}

func mapClusterMessage(m ClusterSearchResult) council.Message {
	return council.Message{
		ID:          m.ID,
		RoomID:      m.RoomID,
		Author:      m.Author,
		Content:     m.Content,
		MessageType: m.MessageType,
		IsSummary:   m.IsSummary,
		ReplyTo:     m.ReplyTo,
		Pinned:      m.Pinned,
		Timestamp:   parseClusterTime(m.Timestamp),
	}
}

func mapClusterRoom(r ClusterRoomResult) council.Room {
	return council.Room{
		ID:           r.ID,
		Description:  r.Description,
		Status:       r.Status,
		Project:      r.Project,
		TechStack:    r.TechStack,
		Tags:         r.Tags,
		SystemPrompt: r.SystemPrompt,
		RelatedRooms: r.RelatedRooms,
		CreatedAt:    parseClusterTime(r.CreatedAt),
		UpdatedAt:    parseClusterTime(r.UpdatedAt),
	}
}

func (r *Registry) handleReadRoomCluster(args ReadRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
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

	var room *ClusterRoomResult
	for _, res := range results {
		if res.ID == args.RoomID {
			room = &res
			break
		}
	}

	if room == nil {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Error: room '%s' not found on any cluster node.", args.RoomID))
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
	fmt.Fprintf(&b, "**Created:** %s\n", room.CreatedAt)
	fmt.Fprintf(&b, "**Updated:** %s\n", room.UpdatedAt)

	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}

func (r *Registry) handleReadTranscriptCluster(args ReadTranscriptInput, roomID string) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

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
		b.WriteString(fmt.Sprintf("Error: room '%s' not found on any cluster node.", roomID))
		formatClusterWarnings(&b, warnings)
		return msg(b.String())
	}

	room := mapClusterRoom(result.Room)
	var messages []council.Message

	// Filter down the cluster messages just like Go does
	limit := 0
	if args.LastN != "" {
		fmt.Sscanf(args.LastN, "%d", &limit)
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
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	params := map[string]any{
		"query":        args.Query,
		"author":       args.Author,
		"message_type": args.MessageType,
		"room_id":      args.RoomID,
		"project":      args.Project,
		"since":        args.Since,
		"until":        args.Until,
		"limit":        args.Limit,
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
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	params := map[string]any{
		"project": args.Project,
		"tag":     args.Tag,
		"status":  args.Status,
		"search":  args.Search,
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
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	params := map[string]any{
		"room_id": args.RoomID,
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
		b.WriteString(fmt.Sprintf("Error: room '%s' not found on any cluster node.", args.RoomID))
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
