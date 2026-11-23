package council

import (
	"database/sql"
	"fmt"
	"time"
)

// Task leases: a claim is a promise to the hub that someone is working on a task
// until a deadline. There is no sweep job. An expired lease is taken over by the
// next claim, and reads show it as stale without changing anything.
//
// Limits: the owner is a string, not an authenticated identity; time is the
// server's clock; a lease says nothing about whether the owner is alive.

const (
	// DefaultLease is used when a claim names no lease length.
	DefaultLease = 900 * time.Second
	// MaxLease caps a single claim or renewal.
	MaxLease = 24 * time.Hour
)

// TaskLease is the lease state of a task entry.
type TaskLease struct {
	Status     string
	Owner      string
	Room       string // task_room: where claim/release/done notes go
	Label      string
	ClaimedAt  time.Time
	LeaseUntil time.Time
}

func (s *Server) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

// Now is the clock task leases use.
func (s *Server) Now() time.Time { return s.now() }

func (s *Server) nowUTC() string { return s.now().UTC().Format(time.RFC3339) }

func parseLease(v string) time.Time {
	t, _ := time.Parse(time.RFC3339, v)
	return t
}

// leaseLeft renders the time remaining on a lease as whole seconds.
func leaseLeft(leaseUntil string, now time.Time) string {
	d := parseLease(leaseUntil).Sub(now.UTC().Truncate(time.Second))
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// TaskLeaseStale reports whether a doing task's lease has passed.
func TaskLeaseStale(status, leaseUntil string, now time.Time) bool {
	return status == "doing" && leaseUntil != "" && leaseUntil <= now.UTC().Format(time.RFC3339)
}

func checkLease(owner string, lease time.Duration) error {
	if owner == "" {
		return fmt.Errorf("owner is required (pass owner=, or connect with a client name)")
	}
	if lease < time.Second || lease > MaxLease {
		return fmt.Errorf("lease_seconds must be between 1 and %d", int(MaxLease.Seconds()))
	}
	return nil
}

// taskLeaseLocked reads one task's lease state. Caller holds s.Mu.
func (s *Server) taskLeaseLocked(entryID string) (TaskLease, string, error) {
	var kind, claimed, until string
	var l TaskLease
	err := s.DB.QueryRow(`SELECT kind, COALESCE(status, 'open'), COALESCE(task_owner, ''), COALESCE(task_room, ''), prose,
		COALESCE(task_claimed_at, ''), COALESCE(task_lease_until, '') FROM notebook_entries WHERE id = ?`, entryID).
		Scan(&kind, &l.Status, &l.Owner, &l.Room, &l.Label, &claimed, &until)
	if err == sql.ErrNoRows {
		return l, "", fmt.Errorf("entry '%s' not found", entryID)
	}
	if err != nil {
		return l, "", err
	}
	if kind != "task" {
		return l, "", fmt.Errorf("entry '%s' is a %s, not a task — only task entries can be claimed", entryID, kind)
	}
	l.ClaimedAt, l.LeaseUntil = parseLease(claimed), parseLease(until)
	return l, until, nil
}

// TaskLeaseInfo returns a task's current lease state without changing it.
func (s *Server) TaskLeaseInfo(entryID string) (TaskLease, error) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	l, _, err := s.taskLeaseLocked(entryID)
	return l, err
}

// ClaimTask takes an open task, or one whose lease has lapsed, for owner until
// now+lease. One UPDATE decides the winner: when two callers race, the second
// matches no row and gets an error naming the holder.
func (s *Server) ClaimTask(entryID, owner string, lease time.Duration) (TaskLease, error) {
	if err := checkLease(owner, lease); err != nil {
		return TaskLease{}, err
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()

	now := s.now().UTC().Truncate(time.Second)
	nowS := now.Format(time.RFC3339)
	until := now.Add(lease).Format(time.RFC3339)
	res, err := s.DB.Exec(`UPDATE notebook_entries
		SET status = 'doing', task_owner = ?, task_claimed_at = ?, task_lease_until = ?
		WHERE id = ? AND kind = 'task'
		  AND (COALESCE(status, 'open') = 'open'
		       OR (status = 'doing' AND COALESCE(task_lease_until, '') <> '' AND task_lease_until <= ?))`,
		owner, nowS, until, entryID, nowS)
	if err != nil {
		return TaskLease{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		l, lu, err := s.taskLeaseLocked(entryID)
		if err != nil {
			return TaskLease{}, err
		}
		switch {
		case l.Status == "done":
			return TaskLease{}, fmt.Errorf("task is done; it cannot be claimed")
		case lu == "":
			return TaskLease{}, fmt.Errorf("task is in progress without a lease (owner %q); uncheck it first or ask its owner", l.Owner)
		default:
			return TaskLease{}, fmt.Errorf("task is held by %s for another %s", l.Owner, leaseLeft(lu, now))
		}
	}
	if err := s.touchEntryNotebookLocked(entryID); err != nil {
		return TaskLease{}, err
	}
	l, _, err := s.taskLeaseLocked(entryID)
	return l, err
}

// RenewTask extends a live lease to now+lease. Only the current owner may, and
// only while the lease is live: a lapsed lease must be claimed again.
func (s *Server) RenewTask(entryID, owner string, lease time.Duration) (TaskLease, error) {
	if err := checkLease(owner, lease); err != nil {
		return TaskLease{}, err
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()

	now := s.now().UTC().Truncate(time.Second)
	nowS := now.Format(time.RFC3339)
	res, err := s.DB.Exec(`UPDATE notebook_entries SET task_lease_until = ?
		WHERE id = ? AND kind = 'task' AND status = 'doing' AND task_owner = ?
		  AND COALESCE(task_lease_until, '') > ?`, now.Add(lease).Format(time.RFC3339), entryID, owner, nowS)
	if err != nil {
		return TaskLease{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return TaskLease{}, s.notHolderLocked(entryID, owner, "renew", now)
	}
	l, _, err := s.taskLeaseLocked(entryID)
	return l, err
}

// ReleaseTask gives a leased task back: status open, owner and lease cleared.
// Only the current owner may, live or lapsed. The returned lease is the state
// before the release (it carries the room for the note).
func (s *Server) ReleaseTask(entryID, owner string) (TaskLease, error) {
	if owner == "" {
		return TaskLease{}, fmt.Errorf("owner is required (pass owner=, or connect with a client name)")
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()

	before, _, err := s.taskLeaseLocked(entryID)
	if err != nil {
		return TaskLease{}, err
	}
	res, err := s.DB.Exec(`UPDATE notebook_entries
		SET status = 'open', task_owner = '', task_claimed_at = '', task_lease_until = ''
		WHERE id = ? AND kind = 'task' AND status = 'doing' AND task_owner = ? AND COALESCE(task_lease_until, '') <> ''`, entryID, owner)
	if err != nil {
		return TaskLease{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return TaskLease{}, s.notHolderLocked(entryID, owner, "release", s.now().UTC().Truncate(time.Second))
	}
	if err := s.touchEntryNotebookLocked(entryID); err != nil {
		return TaskLease{}, err
	}
	return before, nil
}

// notHolderLocked explains why renew or release matched nothing.
func (s *Server) notHolderLocked(entryID, owner, verb string, now time.Time) error {
	l, lu, err := s.taskLeaseLocked(entryID)
	if err != nil {
		return err
	}
	switch {
	case lu == "" || l.Status != "doing":
		return fmt.Errorf("cannot %s: task is not claimed (status %s)", verb, l.Status)
	case l.Owner != owner:
		return fmt.Errorf("cannot %s: task is held by %s, not %s", verb, l.Owner, owner)
	default:
		return fmt.Errorf("cannot %s: your lease lapsed; claim the task again", verb)
	}
}

func (s *Server) touchEntryNotebookLocked(entryID string) error {
	_, err := s.DB.Exec(`UPDATE notebooks SET updated_at = CURRENT_TIMESTAMP WHERE id = (SELECT notebook_id FROM notebook_entries WHERE id = ?)`, entryID)
	return err
}
