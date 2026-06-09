package handlers

import (
	"context"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListRoomsInput represents the parameters for listing rooms.
type ListRoomsInput struct {
	Project      string `json:"project"`
	ProjectNotIn string `json:"project_not_in"`
	Tag          string `json:"tag"`
	Status       string `json:"status"`
	Search       string `json:"search"`
	RelatedTo    string `json:"related_to"`
	Compact      string `json:"compact"` // deprecated: compact is now default; kept for backwards compat
	Verbose      string `json:"verbose"`
	ClusterWide  string `json:"cluster_wide"`
	Limit        string `json:"limit"`
	Offset       string `json:"offset"`
}

// RoomStatsInput represents the parameters for getting room statistics.
type RoomStatsInput struct {
	RoomID      string `json:"room_id"`
	RoomIDs     string `json:"room_ids"`
	ClusterWide string `json:"cluster_wide"`
}

func (r *Registry) handleListRooms(ctx context.Context, req *mcp.CallToolRequest, args ListRoomsInput) (*mcp.CallToolResult, ToolOutput, error) {
	if args.ClusterWide == "true" {
		return r.handleListRoomsCluster(args)
	}

	msg := textResult

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

	rooms, err := r.Server.ListRoomsFiltered(council.ListRoomsOptions{
		Project:      args.Project,
		ProjectNotIn: projectNotIn,
		Tag:          args.Tag,
		Status:       args.Status,
		Search:       args.Search,
		RelatedTo:    args.RelatedTo,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		r.Server.Logger.Error("Failed to list rooms", "error", err)
		return nil, ToolOutput{}, err
	}

	if len(rooms) == 0 {
		return msg("No rooms found matching the given filters.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d room(s):\n\n", len(rooms))

	// Compact is the default. Verbose mode is opt-in via verbose=true.
	// Legacy compact=false maps to verbose for backwards compat.
	// Collect room IDs for batch queries
	roomIDs := make([]string, len(rooms))
	for i, rm := range rooms {
		roomIDs[i] = rm.ID
	}
	msgCounts := r.Server.GetMessageCounts()
	pinnedExcerpts := r.Server.GetPinnedExcerpts(roomIDs)
	latestIDs := r.Server.GetLatestMessageIDs()

	useVerbose := args.Verbose == "true" || args.Compact == "false"
	if !useVerbose {
		for _, rm := range rooms {
			topic := rm.Description
			if len(topic) > 60 {
				topic = topic[:60] + "..."
			}
			project := rm.Project
			if project == "" {
				project = "-"
			}
			count := msgCounts[rm.ID]
			cursor := ""
			if lid, ok := latestIDs[rm.ID]; ok {
				cursor = fmt.Sprintf(" | cursor:%.8s", lid)
			}
			if excerpt, ok := pinnedExcerpts[rm.ID]; ok {
				fmt.Fprintf(&b, "- **%s** | %s | %s | %d msgs%s | 📌 %s | %s | %s\n", rm.ID, project, rm.Status, count, cursor, excerpt, topic, rm.UpdatedAt.Format("2006-01-02 15:04"))
			} else {
				fmt.Fprintf(&b, "- **%s** | %s | %s | %d msgs%s | %s | %s\n", rm.ID, project, rm.Status, count, cursor, topic, rm.UpdatedAt.Format("2006-01-02 15:04"))
			}
		}
	} else {
		for _, rm := range rooms {
			fmt.Fprintf(&b, "- **%s** [%s]", rm.ID, rm.Status)
			if rm.Project != "" {
				fmt.Fprintf(&b, " | project: %s", rm.Project)
			}
			if rm.Tags != "" {
				fmt.Fprintf(&b, " | tags: %s", rm.Tags)
			}
			fmt.Fprintf(&b, "\n  %s\n", rm.Description)
			if excerpt, ok := pinnedExcerpts[rm.ID]; ok {
				fmt.Fprintf(&b, "  📌 %s\n", excerpt)
			}
			if rm.TechStack != "" {
				fmt.Fprintf(&b, "  Tech: %s\n", rm.TechStack)
			}
			if rm.RelatedRooms != "" {
				fmt.Fprintf(&b, "  Related: %s\n", rm.RelatedRooms)
			}
			if lid, ok := latestIDs[rm.ID]; ok {
				fmt.Fprintf(&b, "  Latest msg: %.8s\n", lid)
			}
			fmt.Fprintf(&b, "  Last activity: %s\n", rm.UpdatedAt.Format("2006-01-02 15:04:05"))
		}
	}

	text := b.String()
	return msg(text)
}

func (r *Registry) handleRoomStats(ctx context.Context, req *mcp.CallToolRequest, args RoomStatsInput) (*mcp.CallToolResult, ToolOutput, error) {
	if args.ClusterWide == "true" {
		return r.handleRoomStatsCluster(args)
	}

	msg := textResult

	// Collect target room IDs from room_id and/or room_ids
	seen := map[string]bool{}
	var ids []string
	if args.RoomIDs != "" {
		for _, id := range strings.Split(args.RoomIDs, ",") {
			id = strings.TrimSpace(id)
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if args.RoomID != "" && !seen[args.RoomID] {
		ids = append(ids, args.RoomID)
	}
	if len(ids) == 0 {
		return msg("Error: room_id or room_ids is required.")
	}

	var b strings.Builder
	for i, roomID := range ids {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		stats, err := r.Server.GetRoomStats(roomID)
		if err != nil {
			fmt.Fprintf(&b, "Error: %s\n", err.Error())
			continue
		}

		fmt.Fprintf(&b, "**%s** [%s]\n", stats.RoomID, stats.Status)
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
			fmt.Fprintf(&b, "**First message:** %s\n", stats.FirstMessage.Format("2006-01-02 15:04:05"))
			fmt.Fprintf(&b, "**Last message:** %s\n", stats.LastMessage.Format("2006-01-02 15:04:05"))
		}

		if len(stats.TypeCounts) > 0 {
			var types []string
			for msgType, count := range stats.TypeCounts {
				types = append(types, fmt.Sprintf("%s: %d", msgType, count))
			}
			fmt.Fprintf(&b, "**Types:** %s\n", strings.Join(types, ", "))
		}
	}

	return msg(b.String())
}
