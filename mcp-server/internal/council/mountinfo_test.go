package council

import (
	"strings"
	"testing"
)

const mountinfoDesktop = `1100 1090 0:50 / / rw,relatime - overlay overlay rw,lowerdir=/x
1101 1100 0:53 / /proc rw,nosuid - proc proc rw
1110 1100 0:60 / /data rw,relatime - virtiofs virtiofs rw
1111 1100 0:61 / /cache rw,relatime - ext4 /dev/vda1 rw
`

func TestSharedFSWarning(t *testing.T) {
	cases := []struct {
		name, dir string
		want      bool
	}{
		{"virtiofs bind mount", "/data", true},
		{"file inside bind mount", "/data/sub", true},
		{"named volume on ext4", "/cache", false},
		{"container overlay root", "/srv/db", false},
		{"path merely sharing a prefix", "/database", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sharedFSWarning(mountinfoDesktop, tc.dir)
			if (got != "") != tc.want {
				t.Fatalf("sharedFSWarning(%q) = %q, want warning=%v", tc.dir, got, tc.want)
			}
			if tc.want && !strings.Contains(got, "virtiofs") {
				t.Errorf("warning should name the filesystem, got %q", got)
			}
		})
	}
}

func TestSharedFSWarningGarbageInput(t *testing.T) {
	if got := sharedFSWarning("not a mountinfo\n\n", "/data"); got != "" {
		t.Errorf("unparseable input must yield no warning, got %q", got)
	}
}
