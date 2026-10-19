package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListRoomsInput represents the parameters for listing rooms.
type ListRoomsInput struct {
	Project      string     `json:"project"`
	ProjectNotIn string     `json:"project_not_in"`
	Tag          string     `json:"tag"`
	Status       string     `json:"status"`
	Search       string     `json:"search"`
	RelatedTo    string     `json:"related_to"`
	Compact      string     `json:"compact"` // deprecated: compact is now default; kept for backwards compat
	Verbose      string     `json:"verbose"`
	ClusterWide  StringBool `json:"cluster_wide"`
	Limit        string     `json:"limit"`
	Offset       string     `json:"offset"`
}

// RoomStatsInput represents the parameters for getting room statistics.
type RoomStatsInput struct {
	RoomID      string     `json:"room_id"`
	RoomIDs     string     `json:"room_ids"`
	ClusterWide StringBool `json:"cluster_wide"`
}

func (r *Registry) handleListRooms(ctx context.Context, req *mcp.CallToolRequest, args ListRoomsInput) (*mcp.CallToolResult, ListRoomsOutput, error) {
	if args.ClusterWide == "true" {
		return r.handleListRoomsCluster(args)
	}

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

	// Default view hides resolved rooms (they are most of a long-lived hub and
	// rarely what the caller wants). An explicit status, a keyword search or a
	// related_to lookup is a targeted ask, so those still see everything.
	status := args.Status
	hideResolved := false
	switch {
	case status == "all":
		status = ""
	case status == "" && args.Search == "" && args.RelatedTo == "":
		hideResolved = true
	}
	listOpts := council.ListRoomsOptions{
		Project:      args.Project,
		ProjectNotIn: projectNotIn,
		Tag:          args.Tag,
		Status:       status,
		Search:       args.Search,
		RelatedTo:    args.RelatedTo,
		Limit:        limit,
		Offset:       offset,
	}
	if hideResolved {
		listOpts.ExcludeStatus = "resolved"
	}
	rooms, err := r.Server.ListRoomsFiltered(listOpts)
	if err != nil {
		r.Server.Logger.Error("Failed to list rooms", "error", err)
		return nil, ListRoomsOutput{}, err
	}

	// How many resolved rooms the default view left out, so the omission is visible.
	hiddenNote := ""
	omittedResolved := 0
	if hideResolved {
		resolvedOpts := listOpts
		resolvedOpts.ExcludeStatus, resolvedOpts.Status = "", "resolved"
		if n, cerr := r.Server.CountRooms(resolvedOpts); cerr == nil && n > 0 {
			omittedResolved = n
			hiddenNote = fmt.Sprintf("\n(%d resolved room(s) not shown — pass status=resolved or status=all to include them.)\n", n)
		}
	}

	if len(rooms) == 0 {
		out := newListRoomsOutput(nil, nil)
		out.OmittedResolved = omittedResolved
		return listRoomsResult("No rooms found matching the given filters."+hiddenNote, out)
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
			topic := council.TruncateRunes(rm.Description, 60, "", 0)
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
				fmt.Fprintf(&b, "- **%s** | %s | %s | %d msgs%s | 📌 %s | %s | %s\n", rm.ID, project, rm.Status, count, cursor, excerpt, topic, utcStampMinute(rm.UpdatedAt))
			} else {
				fmt.Fprintf(&b, "- **%s** | %s | %s | %d msgs%s | %s | %s\n", rm.ID, project, rm.Status, count, cursor, topic, utcStampMinute(rm.UpdatedAt))
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
			fmt.Fprintf(&b, "  Last activity: %s\n", utcStamp(rm.UpdatedAt))
		}
	}

	if total, cerr := r.Server.CountRooms(listOpts); cerr == nil && total > offset+len(rooms) {
		fmt.Fprintf(&b, "\n(%d more matching room(s) not shown — raise limit or pass offset=%d.)\n", total-offset-len(rooms), offset+len(rooms))
	}
	b.WriteString(hiddenNote)

	listed := make([]ListedRoom, 0, len(rooms))
	for _, rm := range rooms {
		lr := listedRoomFromCluster(clusterRoomFromLocal(rm))
		lr.PinnedExcerpt = pinnedExcerpts[rm.ID]
		lr.Updated = rm.UpdatedAt.UTC().Format(time.RFC3339)
		listed = append(listed, lr)
	}
	out := newListRoomsOutput(listed, nil)
	out.OmittedResolved = omittedResolved
	return listRoomsResult(b.String(), out)
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
		if stats.PinnedMessageID != "" {
			fmt.Fprintf(&b, "**Pinned:** #%.8s — %d message(s) since pin%s\n",
				stats.PinnedMessageID, stats.MessagesSincePin,
				map[bool]string{true: " ⚠️ pin may be stale", false: ""}[stats.MessagesSincePin >= 5])
		}

		if len(stats.Participants) > 0 {
			var parts []string
			for author, count := range stats.Participants {
				parts = append(parts, fmt.Sprintf("%s (%d)", author, count))
			}
			fmt.Fprintf(&b, "**Participants:** %s\n", strings.Join(parts, ", "))
			fmt.Fprintf(&b, "**First message:** %s\n", utcStamp(stats.FirstMessage))
			fmt.Fprintf(&b, "**Last message:** %s\n", utcStamp(stats.LastMessage))
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
