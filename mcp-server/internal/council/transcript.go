package council

import (
	"fmt"
	"strings"
)

func FormatTranscript(room Room, messages []Message) string {
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
	if room.RelatedRooms != "" {
		fmt.Fprintf(&b, "**Related Rooms:** %s\n", room.RelatedRooms)
	}
	b.WriteString("---\n")

	if room.SystemPrompt != "" {
		fmt.Fprintf(&b, "*Instructions: %s*\n---\n", room.SystemPrompt)
	}

	// Render pinned message first if one exists
	pinnedID := ""
	for _, m := range messages {
		if m.Pinned {
			pinnedID = m.ID
			ts := m.Timestamp.Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "\n**PINNED [#%.8s %s] %s:**\n%s\n---\n", m.ID, ts, m.Author, m.Content)
			break
		}
	}

	for _, m := range messages {
		if m.ID == pinnedID {
			continue // already rendered above
		}
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		replyTag := ""
		if m.ReplyTo != "" {
			replyTag = fmt.Sprintf(", re: #%.8s", m.ReplyTo)
		}
		if m.IsSummary {
			fmt.Fprintf(&b, "\n**[%s] SUMMARY:**\n%s\n", ts, m.Content)
		} else if m.MessageType != "" && m.MessageType != "message" {
			fmt.Fprintf(&b, "\n**[#%.8s %s] %s (%s%s):**\n%s\n", m.ID, ts, m.Author, m.MessageType, replyTag, m.Content)
		} else if m.ReplyTo != "" {
			fmt.Fprintf(&b, "\n**[#%.8s %s] %s (re: #%.8s):**\n%s\n", m.ID, ts, m.Author, m.ReplyTo, m.Content)
		} else {
			fmt.Fprintf(&b, "\n**[#%.8s %s] %s:**\n%s\n", m.ID, ts, m.Author, m.Content)
		}
	}

	b.WriteString("\n---\n")
	fmt.Fprintf(&b, "*SYSTEM: You are reading the Council log for \"%s\". Do not repeat previous points. Use `post_to_room` to contribute your next action.*\n", room.ID)

	return b.String()
}
