package handlers

import (
	"context"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (r *Registry) RegisterResources() {
	r.Server.MCP.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "council://room/{room_id}/transcript",
		Name:        "Room Transcript",
		Description: "Prompt-optimized transcript of a council room, showing summaries and recent messages.",
		MIMEType:    "text/markdown",
	}, r.handleTranscript)
}

func (r *Registry) handleTranscript(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Parse room_id from URI: council://room/{room_id}/transcript
	uri := req.Params.URI
	roomID := strings.TrimPrefix(uri, "council://room/")
	roomID = strings.TrimSuffix(roomID, "/transcript")

	if roomID == "" {
		return nil, fmt.Errorf("invalid URI: missing room_id")
	}

	room, err := r.Server.GetRoom(roomID)
	if err != nil {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: "text/markdown",
				Text:     fmt.Sprintf("Error: Room '%s' not found.", roomID),
			}},
		}, nil
	}

	messages, err := r.Server.GetTranscript(roomID)
	if err != nil {
		r.Server.Logger.Error("Failed to get transcript", "room_id", roomID, "error", err)
		return nil, fmt.Errorf("failed to read transcript: %w", err)
	}

	transcript := council.FormatTranscript(room, messages)

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     transcript,
		}},
	}, nil
}
