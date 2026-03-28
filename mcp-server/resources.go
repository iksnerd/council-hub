package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerResources(cs *CouncilServer) {
	cs.mcp.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "council://room/{room_id}/transcript",
		Name:        "Room Transcript",
		Description: "Prompt-optimized transcript of a council room, showing summaries and recent messages.",
		MIMEType:    "text/markdown",
	}, cs.handleTranscript)
}

func (cs *CouncilServer) handleTranscript(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Parse room_id from URI: council://room/{room_id}/transcript
	uri := req.Params.URI
	roomID := strings.TrimPrefix(uri, "council://room/")
	roomID = strings.TrimSuffix(roomID, "/transcript")

	if roomID == "" {
		return nil, fmt.Errorf("invalid URI: missing room_id")
	}

	room, err := cs.getRoom(roomID)
	if err != nil {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: "text/markdown",
				Text:     fmt.Sprintf("Error: Room '%s' not found.", roomID),
			}},
		}, nil
	}

	messages, err := cs.getTranscript(roomID)
	if err != nil {
		cs.logger.Error("Failed to get transcript", "room_id", roomID, "error", err)
		return nil, fmt.Errorf("failed to read transcript: %w", err)
	}

	transcript := formatTranscript(room, messages)

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     transcript,
		}},
	}, nil
}

func formatTranscript(room Room, messages []Message) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# COUNCIL ROOM: %s\n", room.ID)
	if room.Project != "" {
		fmt.Fprintf(&b, "**Project:** %s\n", room.Project)
	}
	if room.TechStack != "" {
		fmt.Fprintf(&b, "**Tech Stack:** %s\n", room.TechStack)
	}
	fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
	fmt.Fprintf(&b, "**Status:** %s\n", room.Status)
	if room.Tags != "" {
		fmt.Fprintf(&b, "**Tags:** %s\n", room.Tags)
	}
	b.WriteString("---\n")

	if room.SystemPrompt != "" {
		fmt.Fprintf(&b, "*Instructions: %s*\n---\n", room.SystemPrompt)
	}

	for _, m := range messages {
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		if m.IsSummary {
			fmt.Fprintf(&b, "\n**[%s] SUMMARY:**\n%s\n", ts, m.Content)
		} else if m.MessageType != "" && m.MessageType != "message" {
			fmt.Fprintf(&b, "\n**[%s] %s (%s):**\n%s\n", ts, m.Author, m.MessageType, m.Content)
		} else {
			fmt.Fprintf(&b, "\n**[%s] %s:**\n%s\n", ts, m.Author, m.Content)
		}
	}

	b.WriteString("\n---\n")
	fmt.Fprintf(&b, "*SYSTEM: You are reading the Council log for \"%s\". Do not repeat previous points. Use `post_to_room` to contribute your next action.*\n", room.ID)

	return b.String()
}
