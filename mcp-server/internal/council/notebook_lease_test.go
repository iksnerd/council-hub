package council

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// leaseFixture is a server with a controllable clock and one open task.
func leaseFixture(t *testing.T) (*Server, string, *time.Time) {
	t.Helper()
	s := setupTestServer(t)
	seedOutline(t, s)
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	s.Clock = func() time.Time { return now }
	id, err := s.AddOutlineEntry("ol-nb", "task", "", "ship the thing", "")
	if err != nil {
		t.Fatal(err)
	}
	return s, id, &now
}

func entryByID(t *testing.T, s *Server, id string) OutlineEntry {
	t.Helper()
	_, entries, err := s.GetOutline("ol-nb")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("entry %s not found", id)
	return OutlineEntry{}
}

func TestClaimTaskOneWinnerAmongMany(t *testing.T) {
	s, id, _ := leaseFixture(t)
	var wins int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			owner := "agent-" + string(rune('a'+n))
			if _, err := s.ClaimTask(id, owner, time.Minute); err == nil {
				atomic.AddInt32(&wins, 1)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("expected exactly one winner, got %d", wins)
	}
	e := entryByID(t, s, id)
	if e.Status != "doing" || !strings.HasPrefix(e.Owner, "agent-") || e.LeaseUntil == "" || e.ClaimedAt == "" {
		t.Fatalf("claimed task state wrong: %+v", e)
	}
}

func TestClaimTaskBeforeAndAfterExpiry(t *testing.T) {
	s, id, now := leaseFixture(t)
	if _, err := s.ClaimTask(id, "alice", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	*now = now.Add(9 * time.Minute)
	_, err := s.ClaimTask(id, "bob", time.Minute)
	if err == nil || !strings.Contains(err.Error(), "alice") {
		t.Fatalf("claim before expiry must fail naming the owner, got %v", err)
	}
	if !strings.Contains(err.Error(), "60s") {
		t.Errorf("error should state the time left, got %v", err)
	}

	*now = now.Add(2 * time.Minute) // lease lapsed
	l, err := s.ClaimTask(id, "bob", time.Minute)
	if err != nil {
		t.Fatalf("claim after expiry must succeed: %v", err)
	}
	if l.Owner != "bob" {
		t.Errorf("owner after takeover = %q", l.Owner)
	}
	if e := entryByID(t, s, id); e.Owner != "bob" || e.Status != "doing" {
		t.Errorf("takeover state wrong: %+v", e)
	}
}

func TestClaimTaskRefusals(t *testing.T) {
	s, id, _ := leaseFixture(t)
	if _, err := s.ClaimTask(id, "", time.Minute); err == nil {
		t.Error("empty owner must be refused")
	}
	if _, err := s.ClaimTask(id, "a", 0); err == nil {
		t.Error("zero lease must be refused")
	}
	if _, err := s.ClaimTask(id, "a", 25*time.Hour); err == nil {
		t.Error("lease over 24h must be refused")
	}
	if _, err := s.ClaimTask("nope", "a", time.Minute); err == nil {
		t.Error("missing entry must be refused")
	}
	if err := s.SetTaskStatus(id, "done"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimTask(id, "a", time.Minute); err == nil || !strings.Contains(err.Error(), "done") {
		t.Errorf("claiming a done task must say it is done, got %v", err)
	}
	// An unleased task someone started by hand is not claimable.
	id2, _ := s.AddOutlineEntry("ol-nb", "task", "", "manual", "")
	_ = s.SetTaskStatus(id2, "doing")
	if _, err := s.ClaimTask(id2, "a", time.Minute); err == nil {
		t.Error("an unleased doing task must not be claimable")
	}
	// A prose entry is not a task.
	pid, _ := s.AddOutlineEntry("ol-nb", "prose", "", "hello", "")
	if _, err := s.ClaimTask(pid, "a", time.Minute); err == nil {
		t.Error("a non-task entry must be refused")
	}
}

func TestRenewAndReleaseOwnerOnly(t *testing.T) {
	s, id, now := leaseFixture(t)
	if _, err := s.ClaimTask(id, "alice", 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RenewTask(id, "bob", time.Hour); err == nil || !strings.Contains(err.Error(), "alice") {
		t.Errorf("renew by non-owner must be refused naming the owner, got %v", err)
	}
	if _, err := s.ReleaseTask(id, "bob"); err == nil || !strings.Contains(err.Error(), "alice") {
		t.Errorf("release by non-owner must be refused naming the owner, got %v", err)
	}

	*now = now.Add(5 * time.Minute)
	l, err := s.RenewTask(id, "alice", 10*time.Minute)
	if err != nil {
		t.Fatalf("owner renew failed: %v", err)
	}
	if want := now.Add(10 * time.Minute); !l.LeaseUntil.Equal(want) {
		t.Errorf("renew sets lease from now: got %v want %v", l.LeaseUntil, want)
	}

	// Renew after the lease lapsed is refused: the owner must claim again.
	*now = now.Add(11 * time.Minute)
	if _, err := s.RenewTask(id, "alice", time.Minute); err == nil {
		t.Error("renew of a lapsed lease must be refused")
	}

	// Release returns the task to open and clears every lease field.
	*now = now.Add(-11 * time.Minute)
	if _, err := s.ReleaseTask(id, "alice"); err != nil {
		t.Fatalf("owner release failed: %v", err)
	}
	e := entryByID(t, s, id)
	if e.Status != "open" || e.Owner != "" || e.LeaseUntil != "" || e.ClaimedAt != "" {
		t.Errorf("release must reset the task: %+v", e)
	}
	if _, err := s.ReleaseTask(id, "alice"); err == nil {
		t.Error("releasing an unclaimed task must be refused")
	}
	if _, err := s.ClaimTask(id, "bob", time.Minute); err != nil {
		t.Errorf("a released task must be claimable: %v", err)
	}
}

func TestStartCheckByNonOwnerOnLiveLeaseRefused(t *testing.T) {
	s, id, now := leaseFixture(t)
	if _, err := s.ClaimTask(id, "alice", 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, st := range []string{"doing", "done", "open"} {
		err := s.SetTaskStatusAs(id, st, "bob")
		if err == nil || !strings.Contains(err.Error(), "alice") {
			t.Errorf("%s by non-owner on a live lease must be refused naming the owner, got %v", st, err)
		}
	}
	if err := s.SetTaskStatusAs(id, "doing", "alice"); err != nil {
		t.Errorf("start by owner must pass: %v", err)
	}
	if e := entryByID(t, s, id); e.LeaseUntil == "" {
		t.Error("start by the owner must keep the lease")
	}

	// After expiry anyone may move it; the stale lease is cleared.
	*now = now.Add(time.Hour)
	if err := s.SetTaskStatusAs(id, "done", "bob"); err != nil {
		t.Fatalf("check after expiry must pass: %v", err)
	}
	if e := entryByID(t, s, id); e.Status != "done" || e.LeaseUntil != "" {
		t.Errorf("done must clear the lease: %+v", e)
	}
}

func TestUnleasedTaskBehavesAsBefore(t *testing.T) {
	s, id, _ := leaseFixture(t)
	for _, st := range []string{"doing", "done", "open"} {
		if err := s.SetTaskStatusAs(id, st, "anyone"); err != nil {
			t.Errorf("unleased %s must pass: %v", st, err)
		}
		if err := s.SetTaskStatusAs(id, st, ""); err != nil {
			t.Errorf("unleased %s with no identity must pass: %v", st, err)
		}
	}
	e := entryByID(t, s, id)
	if e.LeaseUntil != "" || e.ClaimedAt != "" {
		t.Errorf("unleased task grew lease fields: %+v", e)
	}
}

func TestTaskLeaseStale(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute).Format(time.RFC3339)
	future := now.Add(time.Minute).Format(time.RFC3339)
	cases := []struct {
		status, lease string
		want          bool
	}{
		{"doing", past, true},
		{"doing", future, false},
		{"doing", "", false},
		{"open", past, false},
		{"done", past, false},
	}
	for _, c := range cases {
		if got := TaskLeaseStale(c.status, c.lease, now); got != c.want {
			t.Errorf("TaskLeaseStale(%q,%q) = %v, want %v", c.status, c.lease, got, c.want)
		}
	}
}

func TestLeaseMigrationRunsTwice(t *testing.T) {
	path := t.TempDir() + "/lease.db"
	s, err := NewServer(path, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.DB.Close()
	s2, err := NewServer(path, testLogger())
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	t.Cleanup(func() { _ = s2.DB.Close() })
	var n int
	if err := s2.DB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('notebook_entries') WHERE name IN ('task_claimed_at','task_lease_until')`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("lease columns missing after reopen: n=%d err=%v", n, err)
	}
}
