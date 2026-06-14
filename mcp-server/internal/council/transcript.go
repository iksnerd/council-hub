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

// headerPrefix builds the "[#id ts] author" lead-in for a message, honoring the
// view's metadata toggles. Returns "" when all three are hidden.
func headerPrefix(m Message, v ViewSpec, includeID bool) string {
	var br []string
	if v.ShowIDs && includeID {
		br = append(br, fmt.Sprintf("#%.8s", m.ID))
	}
	if v.ShowTimestamps {
		br = append(br, m.Timestamp.Format("2006-01-02 15:04:05"))
	}
	prefix := ""
	if len(br) > 0 {
		prefix = "[" + strings.Join(br, " ") + "]"
	}
	if v.ShowAuthor {
		if prefix != "" {
			prefix += " "
		}
		prefix += m.Author
	}
	return prefix
}

// FormatTranscript renders a room with the default view (everything shown, full bodies).
func FormatTranscript(room Room, messages []Message) string {
	return FormatTranscriptView(room, messages, DefaultViewSpec())
}

// FormatTranscriptView renders a room projected through a ViewSpec.
func FormatTranscriptView(room Room, messages []Message, v ViewSpec) string {
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

	// Reverse supersedes index: supersededBy[old.id] = new.id. Lets a superseded
	// message show it's been replaced (the backlink), not just the replacement
	// pointing back — so a stale pin reads as dead at a glance.
	supersededBy := make(map[string]string)
	for _, m := range messages {
		if m.Supersedes != "" {
			supersededBy[m.Supersedes] = m.ID
		}
	}

	projectBody := func(raw string) string {
		c := ResolveCommitRefs(raw, room.Repo)
		if v.TruncateLineOne {
			c = firstLine(c)
		}
		return c
	}

	// Render pinned message first if one exists
	pinnedID := ""
	for _, m := range messages {
		if m.Pinned {
			pinnedID = m.ID
			staleNote := ""
			if by, ok := supersededBy[m.ID]; ok {
				staleNote = fmt.Sprintf(" ⚠️ superseded by #%.8s", by)
			}
			if m.Revises != "" {
				staleNote += " ✎ edited"
			}
			prefix := headerPrefix(m, v, true)
			if prefix != "" {
				prefix = " " + prefix
			}
			pinnedBody := projectBody(m.Content)
			if m.RetractedAt.Valid {
				pinnedBody = DisplayContent(m)
			}
			fmt.Fprintf(&b, "\n**PINNED%s%s:**\n%s\n---\n", prefix, staleNote, pinnedBody)
			break
		}
	}

	for _, m := range messages {
		if m.ID == pinnedID {
			continue // already rendered above
		}
		content := projectBody(m.Content)
		// A retracted node survives in the log but its content reads as a tombstone —
		// the immutable counterpart to deletion (graph and lineage stay intact).
		if m.RetractedAt.Valid {
			content = DisplayContent(m)
		}
		// A head revision carries an "edited" marker; its prior versions are hidden
		// from the transcript (filtered to revised = 0) but remain in the graph.
		editedTag := ""
		if m.Revises != "" {
			editedTag = " ✎ edited"
		}
		replyTag := ""
		if m.ReplyTo != "" {
			replyTag = fmt.Sprintf(", re: #%.8s", m.ReplyTo)
		}
		supersedesTag := ""
		if m.Supersedes != "" {
			supersedesTag = fmt.Sprintf(", supersedes #%.8s", m.Supersedes)
		}
		if by, ok := supersededBy[m.ID]; ok {
			supersedesTag += fmt.Sprintf(", superseded by #%.8s", by)
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
			bracket := ""
			if v.ShowTimestamps {
				bracket = "[" + m.Timestamp.Format("2006-01-02 15:04:05") + "] "
			}
			fmt.Fprintf(&b, "\n**%sSUMMARY:**\n%s\n", bracket, content)
		} else {
			prefix := headerPrefix(m, v, true)
			var annot string
			if m.MessageType != "" && m.MessageType != "message" {
				annot = " (" + m.MessageType + replyTag + supersedesTag + ")"
			} else if m.ReplyTo != "" {
				annot = fmt.Sprintf(" (re: #%.8s%s)", m.ReplyTo, supersedesTag)
			} else {
				annot = supersedesTag // bare, no parens (matches the plain-message form)
			}
			header := strings.TrimLeft(prefix+annot+mentionTag+editedTag, " ")
			fmt.Fprintf(&b, "\n**%s:**\n%s\n", header, content)
		}
		if v.ShowReactions {
			if r := formatReactions(m.Reactions); r != "" {
				fmt.Fprintf(&b, "  Reactions: %s\n", r)
			}
		}
	}

	b.WriteString("\n---\n")
	fmt.Fprintf(&b, "*SYSTEM: You are reading the Council log for \"%s\". Do not repeat previous points. Use `post_to_room` to contribute your next action.*\n", room.ID)

	return b.String()
}
