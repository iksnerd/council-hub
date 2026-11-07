package tailnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Config is the operator's choice, saved next to the database so it survives a
// restart. It never holds an auth key: a key is used once and not stored.
type Config struct {
	Enabled    bool     `json:"enabled"`
	Hostname   string   `json:"hostname"`
	Access     Access   `json:"access"`
	ControlURL string   `json:"control_url,omitempty"` // empty means Tailscale's own control server
	Tags       []string `json:"tags,omitempty"`        // e.g. "tag:council"; a tagged device has no owner
}

// DefaultConfig is off, owner-only, and named council-hub.
func DefaultConfig() Config {
	return Config{Hostname: "council-hub", Access: AccessOwner}
}

var (
	hostnameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	tagRe      = regexp.MustCompile(`^tag:[a-z][a-z0-9-]*$`)
)

// Validate rejects a config that could not be applied or that would widen
// access by accident. A tagged device has no owner, so tags need members mode.
func (c Config) Validate() error {
	if !hostnameRe.MatchString(c.Hostname) {
		return errors.New("hostname must be 1-63 lowercase letters, digits or hyphens")
	}
	if !c.Access.Valid() {
		return fmt.Errorf("access must be %q or %q", AccessOwner, AccessMembers)
	}
	if c.ControlURL != "" && !strings.HasPrefix(c.ControlURL, "https://") && !strings.HasPrefix(c.ControlURL, "http://") {
		return errors.New("control_url must start with http:// or https://")
	}
	for _, t := range c.Tags {
		if !tagRe.MatchString(t) {
			return fmt.Errorf("tag %q must look like tag:name", t)
		}
	}
	if len(c.Tags) > 0 && c.Access == AccessOwner {
		return errors.New("a tagged device has no owner: tags need access \"members\"")
	}
	return nil
}

// Store reads and writes the config file.
type Store struct{ Path string }

// Load returns the saved config, or DefaultConfig when none exists. A file that
// does not parse or validate is an error, not a silent reset: resetting would
// quietly change who can connect.
func (s Store) Load() (Config, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	c := DefaultConfig()
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", s.Path, err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", s.Path, err)
	}
	return c, nil
}

// Save writes the config atomically (temp file, then rename) with mode 0600.
func (s Store) Save(c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".tailscale-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.Path)
}
