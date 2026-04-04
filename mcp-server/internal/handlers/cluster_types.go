package handlers

import (
	"fmt"
	"strings"
	"time"

	"council-hub/internal/council"
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

type ClusterDigestResult struct {
	RoomID               string `json:"room_id"`
	NewMessageCount      int    `json:"new_message_count"`
	LatestMessageExcerpt string `json:"latest_message_excerpt"`
	SourceNode           string `json:"source_node"`
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
