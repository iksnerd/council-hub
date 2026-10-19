package handlers

import "time"

// utcStamp renders a stored timestamp (always UTC) as ISO-8601 with an explicit
// Z, so an agent comparing it to local `date` output cannot misread the zone.
func utcStamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

// utcStampMinute is utcStamp without seconds, for the compact room listing.
func utcStampMinute(t time.Time) string { return t.UTC().Format("2006-01-02T15:04Z") }
