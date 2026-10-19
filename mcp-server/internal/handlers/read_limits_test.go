package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var utcStampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2})?Z`)

func TestGetMessagesLimitBoundsDeltaRead(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "lim-room")
	var ids []string
	for i := 0; i < 8; i++ {
		ids = append(ids, mustPost(t, reg.Server, "lim-room", "a", fmt.Sprintf("body-%d", i)))
	}
	res, _, err := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{RoomID: "lim-room", AfterID: ids[0], Limit: "3"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(res)
	if !strings.Contains(text, "body-1") || !strings.Contains(text, "body-3") || strings.Contains(text, "body-4") {
		t.Errorf("limit=3 after first should return bodies 1..3 only:\n%s", text)
	}
	if !strings.Contains(text, "more") || !strings.Contains(text, "after_id="+ids[3]) {
		t.Errorf("expected a continuation cursor naming %s:\n%s", ids[3], text)
	}
}

func TestGetMessagesBeforeID(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "before-room")
	var ids []string
	for i := 0; i < 6; i++ {
		ids = append(ids, mustPost(t, reg.Server, "before-room", "a", fmt.Sprintf("body-%d", i)))
	}
	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{RoomID: "before-room", BeforeID: ids[4], Limit: "2"})
	text := resultText(res)
	if !strings.Contains(text, "body-2") || !strings.Contains(text, "body-3") || strings.Contains(text, "body-1") || strings.Contains(text, "body-4") {
		t.Errorf("before_id=ids[4] limit=2 should return bodies 2,3:\n%s", text)
	}
}

func TestGetMessagesDeltaReadHasDefaultCap(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "cap-room")
	first := mustPost(t, reg.Server, "cap-room", "a", "first")
	for i := 0; i < 60; i++ {
		mustPost(t, reg.Server, "cap-room", "a", fmt.Sprintf("m-%d", i))
	}
	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{RoomID: "cap-room", AfterID: first})
	text := resultText(res)
	if !strings.Contains(text, "Found 50 message(s)") || !strings.Contains(text, "after_id=") {
		t.Errorf("unbounded delta read must cap at 50 and give a cursor:\n%.200s", text)
	}
}

func TestReadRoomAcceptsLimitAlias(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "rr-lim")
	for i := 0; i < 5; i++ {
		mustPost(t, reg.Server, "rr-lim", "a", fmt.Sprintf("rr-body-%d", i))
	}
	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "rr-lim", Limit: "2"})
	text := resultText(res)
	if !strings.Contains(text, "rr-body-4") || !strings.Contains(text, "rr-body-3") || strings.Contains(text, "rr-body-2") {
		t.Errorf("limit=2 should inline the last two messages:\n%s", text)
	}
}

func TestListRoomsHidesResolvedByDefault(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "lr-active")
	mustCreateRoom(t, reg.Server, "lr-paused")
	mustCreateRoom(t, reg.Server, "lr-done")
	_ = reg.Server.UpdateStatus("lr-paused", "paused")
	_ = reg.Server.UpdateStatus("lr-done", "resolved")

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if !strings.Contains(text, "lr-active") || !strings.Contains(text, "lr-paused") || strings.Contains(text, "lr-done") {
		t.Errorf("default should list active+paused only:\n%s", text)
	}
	if !strings.Contains(text, "1 resolved room(s) not shown") {
		t.Errorf("should say how many were left out:\n%s", text)
	}

	for _, st := range []string{"resolved", "all"} {
		res, _, _ = reg.handleListRooms(context.Background(), nil, ListRoomsInput{Status: st})
		if !strings.Contains(resultText(res), "lr-done") {
			t.Errorf("status=%s should include the resolved room:\n%s", st, resultText(res))
		}
	}
}

func TestRoomTimesAreLabelledUTC(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "tz-room")
	mustPost(t, reg.Server, "tz-room", "a", "hello")

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{})
	if !utcStampRe.MatchString(resultText(res)) {
		t.Errorf("list_rooms times need a Z suffix:\n%s", resultText(res))
	}
	res, _, _ = reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "tz-room", IncludeLastN: "5"})
	text := resultText(res)
	if !utcStampRe.MatchString(text) || !strings.Contains(text, "T") {
		t.Errorf("read_room times need ISO-8601 Z:\n%s", text)
	}
	if strings.Contains(text, "**Created:** 20") && !strings.Contains(text, "Z\n") {
		t.Errorf("Created/Updated lack Z:\n%s", text)
	}
}
