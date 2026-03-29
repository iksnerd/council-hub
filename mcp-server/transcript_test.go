package main

import (
	"strings"
	"testing"
)

func TestTranscriptFormatting(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("fmt-room", "Formatting test", "", "", "", "", "")
	cs.postMessage("fmt-room", "Claude", "First message", "message", 0)
	cs.postMessage("fmt-room", "Gemini", "Second message", "message", 0)

	room, _ := cs.getRoom("fmt-room")
	msgs, _ := cs.getTranscript("fmt-room")
	transcript := formatTranscript(room, msgs)

	if !strings.Contains(transcript, "# COUNCIL ROOM: fmt-room") {
		t.Error("transcript missing room header")
	}
	if !strings.Contains(transcript, "**Topic:** Formatting test") {
		t.Error("transcript missing topic")
	}
	if !strings.Contains(transcript, "**Status:** active") {
		t.Error("transcript missing status")
	}
	if !strings.Contains(transcript, "Claude") || !strings.Contains(transcript, "Gemini") {
		t.Error("transcript missing authors")
	}
	if !strings.Contains(transcript, "post_to_room") {
		t.Error("transcript missing system instruction")
	}
}

func TestTranscriptWithFullMetadata(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("rich-room", "JWT refactoring", "llm-memory", "Go, SQLite", "auth,security", "Focus on token handling.", "")
	cs.postMessage("rich-room", "Claude", "I think we should use RS256", "thought", 0)
	cs.postMessage("rich-room", "Gemini", "Agreed, let's proceed", "decision", 0)

	room, _ := cs.getRoom("rich-room")
	msgs, _ := cs.getTranscript("rich-room")
	transcript := formatTranscript(room, msgs)

	if !strings.Contains(transcript, "**Project:** llm-memory") {
		t.Error("transcript missing project")
	}
	if !strings.Contains(transcript, "**Tech Stack:** Go, SQLite") {
		t.Error("transcript missing tech stack")
	}
	if !strings.Contains(transcript, "**Tags:** auth,security") {
		t.Error("transcript missing tags")
	}
	if !strings.Contains(transcript, "*Instructions: Focus on token handling.*") {
		t.Error("transcript missing system prompt")
	}
	if !strings.Contains(transcript, "Claude (thought)") {
		t.Error("transcript missing message type for thought")
	}
	if !strings.Contains(transcript, "Gemini (decision)") {
		t.Error("transcript missing message type for decision")
	}
}

func TestTranscriptWithSummary(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("sum-room", "Summary test", "", "", "", "", "")

	for i := 0; i < 5; i++ {
		cs.postMessage("sum-room", "Claude", "Old message", "message", 0)
	}

	cs.insertSummary("sum-room", "Summary of 5 old messages")

	cs.postMessage("sum-room", "Gemini", "New message after summary", "message", 0)

	msgs, err := cs.getTranscript("sum-room")
	if err != nil {
		t.Fatalf("getTranscript failed: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (summary + new), got %d", len(msgs))
	}

	if !msgs[0].IsSummary {
		t.Error("first message should be the summary")
	}
	if msgs[1].Author != "Gemini" {
		t.Errorf("second message should be from Gemini, got '%s'", msgs[1].Author)
	}
}

func TestTranscriptWithRelatedRooms(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("linked-room", "Linked test", "proj", "", "", "", "other-room,another-room")
	cs.postMessage("linked-room", "Claude", "Test", "message", 0)

	room, _ := cs.getRoom("linked-room")
	msgs, _ := cs.getTranscript("linked-room")
	transcript := formatTranscript(room, msgs)

	if !strings.Contains(transcript, "**Related Rooms:** other-room,another-room") {
		t.Error("transcript missing related rooms")
	}
}

func TestJanitorSweep(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("janitor-room", "Janitor test", "", "", "", "", "")

	for i := 0; i < 25; i++ {
		cs.postMessage("janitor-room", "Claude", "Message content", "message", 0)
	}

	rooms, err := cs.getRoomsNeedingSummary(20)
	if err != nil {
		t.Fatalf("getRoomsNeedingSummary failed: %v", err)
	}
	if len(rooms) != 1 || rooms[0] != "janitor-room" {
		t.Fatalf("expected janitor-room to need summary, got %v", rooms)
	}

	cs.janitorSweep()

	msgs, _ := cs.getTranscript("janitor-room")
	hasSummary := false
	for _, m := range msgs {
		if m.IsSummary {
			hasSummary = true
			break
		}
	}
	if !hasSummary {
		t.Error("expected summary message after janitor sweep")
	}

	rooms, _ = cs.getRoomsNeedingSummary(20)
	if len(rooms) != 0 {
		t.Errorf("expected no rooms needing summary after sweep, got %v", rooms)
	}
}

func TestSummarize(t *testing.T) {
	msgs := []Message{
		{Author: "Claude", Content: "First point", MessageType: "thought"},
		{Author: "Gemini", Content: "Second point", MessageType: "decision"},
	}

	summary := summarize(msgs)

	if !strings.Contains(summary, "2 messages") {
		t.Error("summary should mention message count")
	}
	if !strings.Contains(summary, "Claude") || !strings.Contains(summary, "Gemini") {
		t.Error("summary should mention participants")
	}
}

func TestSummarizeLongContent(t *testing.T) {
	longContent := strings.Repeat("A", 300)
	msgs := []Message{
		{Author: "Claude", Content: longContent, MessageType: "message"},
	}

	summary := summarize(msgs)

	// Summarize truncates to 200 chars + "..."
	if !strings.Contains(summary, "...") {
		t.Error("summary should truncate long content")
	}
	if !strings.Contains(summary, "1 messages") {
		t.Error("summary should mention message count")
	}
}
