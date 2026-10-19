package council

import (
	"fmt"
	"os"
)

// BackupTo writes a consistent snapshot of the live database to dest using
// SQLite's VACUUM INTO, which is safe against a running server (it reads under
// a normal read transaction, WAL included) and yields a compacted, standalone
// file. It refuses to overwrite an existing file. Run it inside the container
// (docker exec ... -backup /data/council-backup.db) rather than copying
// council.db from a bind-mounted host directory: see mountinfo.go.
func (s *Server) BackupTo(dest string) error {
	if dest == "" {
		return fmt.Errorf("backup destination is required")
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("backup destination %q already exists", dest)
	}
	if _, err := s.DB.Exec(`VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("backup to %q: %w", dest, err)
	}
	return nil
}
