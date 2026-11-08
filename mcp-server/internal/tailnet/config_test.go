package tailnet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	good := DefaultConfig()
	if err := good.Validate(); err != nil {
		t.Fatalf("default config must validate: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Config)
		want   string // substring of the error
	}{
		{"uppercase hostname", func(c *Config) { c.Hostname = "Council" }, "hostname"},
		{"empty hostname", func(c *Config) { c.Hostname = "" }, "hostname"},
		{"hostname with dot", func(c *Config) { c.Hostname = "a.b" }, "hostname"},
		{"leading hyphen", func(c *Config) { c.Hostname = "-a" }, "hostname"},
		{"unknown access", func(c *Config) { c.Access = "all" }, "access"},
		{"bad control url", func(c *Config) { c.ControlURL = "headscale.example" }, "control_url"},
		{"bad tag", func(c *Config) { c.Tags = []string{"council"}; c.Access = AccessMembers }, "tag"},
		{"tags with owner access", func(c *Config) { c.Tags = []string{"tag:council"} }, "owner"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := DefaultConfig()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.want)
			}
		})
	}

	ok := DefaultConfig()
	ok.Access = AccessMembers
	ok.Tags = []string{"tag:council"}
	ok.ControlURL = "https://headscale.example.net"
	if err := ok.Validate(); err != nil {
		t.Fatalf("tags with members access and an https control url must validate: %v", err)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "sub", "tailscale.json")}

	got, err := s.Load()
	if err != nil || got.Enabled || got.Access != AccessOwner {
		t.Fatalf("missing file must load the default: %+v, %v", got, err)
	}

	want := DefaultConfig()
	want.Enabled = true
	want.Hostname = "hub-two"
	want.Access = AccessMembers
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil || got.Hostname != "hub-two" || !got.Enabled || got.Access != AccessMembers {
		t.Fatalf("round trip lost data: %+v, %v", got, err)
	}

	fi, err := os.Stat(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, want 0600", fi.Mode().Perm())
	}
}

func TestStoreRefusesBadFileInsteadOfResetting(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tailscale.json")
	for name, body := range map[string]string{
		"not json":       "{",
		"unknown access": `{"enabled":true,"hostname":"x","access":"all"}`,
	} {
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := (Store{Path: p}).Load(); err == nil {
			t.Fatalf("%s: Load must fail, not fall back to defaults", name)
		}
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "tailscale.json")}
	bad := DefaultConfig()
	bad.Hostname = "NO"
	if err := s.Save(bad); err == nil {
		t.Fatal("Save must reject an invalid config")
	}
	if _, err := os.Stat(s.Path); err == nil {
		t.Fatal("a rejected save must not create the file")
	}
}
