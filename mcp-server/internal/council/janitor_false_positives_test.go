package council

import "testing"

// backdateRoom shifts every message in a room into the past so grace periods pass.
func backdateRoom(cs *Server, roomID, offset string) {
	cs.DB.Exec(`UPDATE messages SET timestamp = datetime('now', ?) WHERE room_id = ?`, offset, roomID)
}

func roomHasTag(t *testing.T, cs *Server, id, tag string) bool {
	t.Helper()
	room, err := cs.GetRoom(id)
	if err != nil {
		t.Fatal(err)
	}
	return hasTag(room.Tags, tag)
}

func TestLintStalePlansSkipsFreshPlan(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "plan-fresh")
	mustPostTyped(t, cs, "plan-fresh", "Claude", "plan just posted", "plan")
	cs.lintStalePlans()
	if roomHasTag(t, cs, "plan-fresh", "stale-plan") {
		t.Error("a plan inside the grace period must not be flagged")
	}
}

func TestLintStalePlansSkipsWhenLaterSynthesisOrDecision(t *testing.T) {
	for _, typ := range []string{"synthesis", "decision"} {
		cs := setupTestServer(t)
		mustCreateRoom(t, cs, "plan-moved-on")
		mustPostTyped(t, cs, "plan-moved-on", "Claude", "the plan", "plan")
		mustPostTyped(t, cs, "plan-moved-on", "Claude", "we went another way", typ)
		backdateRoom(cs, "plan-moved-on", "-3 days")
		cs.lintStalePlans()
		if roomHasTag(t, cs, "plan-moved-on", "stale-plan") {
			t.Errorf("a later %s supersedes the plan; room must not be flagged", typ)
		}
	}
}

func TestLintStalePlansSkipsExplicitlySupersededPlan(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "plan-superseded")
	planID := mustPostTyped(t, cs, "plan-superseded", "Claude", "old plan", "plan")
	if _, err := cs.PostMessageWithRefs("plan-superseded", "Claude", "replacement", "note", "", "", planID); err != nil {
		t.Fatal(err)
	}
	backdateRoom(cs, "plan-superseded", "-3 days")
	cs.lintStalePlans()
	if roomHasTag(t, cs, "plan-superseded", "stale-plan") {
		t.Error("an explicitly superseded plan must not be flagged")
	}
}

func TestLintStalePlansSkipsRoomWithRecentActivity(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "plan-live")
	mustPostTyped(t, cs, "plan-live", "Claude", "the plan", "plan")
	backdateRoom(cs, "plan-live", "-3 days")
	mustPost(t, cs, "plan-live", "Claude", "working on it right now")
	cs.lintStalePlans()
	if roomHasTag(t, cs, "plan-live", "stale-plan") {
		t.Error("a room with activity in the last day is live, not stale-plan")
	}
}

func TestLinterPostsAreNotActivity(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "sys-noise")
	mustPost(t, cs, "sys-noise", "Claude", "old")
	backdateRoom(cs, "sys-noise", "-8 days")
	mustPost(t, cs, "sys-noise", "system", "### Knowledge Linter\nsomething")
	cs.DB.Exec(`UPDATE rooms SET created_at = datetime('now', '-9 days') WHERE id = 'sys-noise'`)
	cs.lintStaleRooms()
	if !roomHasTag(t, cs, "sys-noise", "stale") {
		t.Error("a fresh linter post must not keep an idle room from going stale")
	}
}

func TestFlagAndNotifySkipsPausedRoom(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "parked")
	mustPost(t, cs, "parked", "Claude", "old")
	if err := cs.UpdateStatus("parked", "paused"); err != nil {
		t.Fatal(err)
	}
	if cs.flagAndNotify("parked", "", "stale", "x") {
		t.Error("flagAndNotify must refuse a paused room")
	}
	msgs, _ := cs.GetRecentMessages("parked", 10)
	for _, m := range msgs {
		if m.Author == "system" {
			t.Error("no linter notice may land in a paused room")
		}
	}
}

func TestJanitorSweepReconcilesStatusTags(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "drift", withTags("resolved,synthesis-complete,keep"))
	mustCreateRoom(t, cs, "truly-done", withTags("resolved,keep"))
	cs.DB.Exec(`UPDATE rooms SET status = 'resolved' WHERE id = 'truly-done'`)
	cs.JanitorSweep()
	room, _ := cs.GetRoom("drift")
	if room.Tags != "keep" {
		t.Errorf("active room should lose resolved/synthesis-complete, got %q", room.Tags)
	}
	done, _ := cs.GetRoom("truly-done")
	if !hasTag(done.Tags, "resolved") {
		t.Errorf("a resolved room keeps its resolved tag, got %q", done.Tags)
	}
}

func TestStripManagedTags(t *testing.T) {
	clean, removed := StripManagedTags("keep, stale-plan ,resolved,other")
	if clean != "keep,other" || len(removed) != 2 {
		t.Errorf("got %q removed %v", clean, removed)
	}
	if got := LinterTagsIn("a,stale,resolved,incoherent"); len(got) != 2 {
		t.Errorf("LinterTagsIn = %v", got)
	}
}
