package council

import (
	"os"
	"path/filepath"
	"strings"
)

// sharedFSTypes are filesystem types that mean a directory is shared with, or
// translated from, another kernel: a Docker Desktop bind mount (virtiofs,
// grpcfuse, osxfs), a network share, or a VM folder. SQLite WAL needs shared
// memory (-shm) that is coherent between every opener; across a VM boundary it
// is not, so opening the same file from the host while the container runs can
// read stale pages or corrupt the database.
var sharedFSTypes = map[string]bool{
	"virtiofs": true, "fuse.grpcfuse": true, "grpcfuse": true, "fuse.osxfs": true,
	"fakeowner": true, "9p": true, "vboxsf": true, "cifs": true, "smb3": true,
	"nfs": true, "nfs4": true, "fuse.sshfs": true, "fuse.rclone": true,
}

// SharedFSWarning inspects /proc/self/mountinfo (Linux only; best-effort) and
// returns a human-readable warning when dbPath's directory sits on a shared
// filesystem, or "" when it does not or the check is unavailable. Never fatal.
func SharedFSWarning(dbPath string) string {
	if dbPath == "" || dbPath == ":memory:" {
		return ""
	}
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	dir, err := filepath.Abs(filepath.Dir(dbPath))
	if err != nil {
		return ""
	}
	return sharedFSWarning(string(data), dir)
}

// sharedFSWarning is the pure core: it picks the mount with the longest
// mount point containing dir and warns if that mount's type is shared.
func sharedFSWarning(mountinfo, dir string) string {
	bestMount, bestType := "", ""
	for _, line := range strings.Split(mountinfo, "\n") {
		// "<id> <parent> <maj:min> <root> <mountpoint> <opts> [optional...] - <fstype> <source> <super opts>"
		fields := strings.Fields(line)
		sep := -1
		for i, f := range fields {
			if f == "-" {
				sep = i
				break
			}
		}
		if sep < 6 || sep+1 >= len(fields) {
			continue
		}
		mp, fstype := fields[4], fields[sep+1]
		if dir != mp && !strings.HasPrefix(dir, strings.TrimSuffix(mp, "/")+"/") {
			continue
		}
		if len(mp) > len(bestMount) {
			bestMount, bestType = mp, fstype
		}
	}
	if !sharedFSTypes[bestType] {
		return ""
	}
	return "database directory " + dir + " is on a " + bestType + " mount (" + bestMount +
		"), which looks like a host bind mount. Do not open this SQLite file from the host while the " +
		"server runs: WAL shared memory does not work across the VM boundary and can corrupt it. " +
		"Use a named volume, or take backups with `-backup` inside the container."
}
