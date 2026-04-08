package council

import (
	"encoding/json"
	"fmt"
	"strings"
)

// formatReactions returns a compact inline reaction string like "👍 3  🎉 1", or "" if none.
func formatReactions(reactionsJSON string) string {
	if reactionsJSON == "" || reactionsJSON == "{}" {
		return ""
	}
	var reactions map[string][]string
	if err := json.Unmarshal([]byte(reactionsJSON), &reactions); err != nil || len(reactions) == 0 {
		return ""
	}
	var parts []string
	for emoji, authors := range reactions {
		parts = append(parts, fmt.Sprintf("%s %d", emoji, len(authors)))
	}
	return strings.Join(parts, "  ")
}

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
		mentionTag := ""
		if m.Mentions != "" {
			var atNames []string
			for _, name := range strings.Split(m.Mentions, ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					atNames = append(atNames, "@"+name)
				}
			}
			if len(atNames) > 0 {
				mentionTag = fmt.Sprintf(" → %s", strings.Join(atNames, ", "))
			}
		}
		if m.IsSummary {
			fmt.Fprintf(&b, "\n**[%s] SUMMARY:**\n%s\n", ts, m.Content)
		} else if m.MessageType != "" && m.MessageType != "message" {
			fmt.Fprintf(&b, "\n**[#%.8s %s] %s (%s%s)%s:**\n%s\n", m.ID, ts, m.Author, m.MessageType, replyTag, mentionTag, m.Content)
		} else if m.ReplyTo != "" {
			fmt.Fprintf(&b, "\n**[#%.8s %s] %s (re: #%.8s)%s:**\n%s\n", m.ID, ts, m.Author, m.ReplyTo, mentionTag, m.Content)
		} else {
			fmt.Fprintf(&b, "\n**[#%.8s %s] %s%s:**\n%s\n", m.ID, ts, m.Author, mentionTag, m.Content)
		}
		if r := formatReactions(m.Reactions); r != "" {
			fmt.Fprintf(&b, "  Reactions: %s\n", r)
		}
	}

	b.WriteString("\n---\n")
	fmt.Fprintf(&b, "*SYSTEM: You are reading the Council log for \"%s\". Do not repeat previous points. Use `post_to_room` to contribute your next action.*\n", room.ID)

	return b.String()
}
