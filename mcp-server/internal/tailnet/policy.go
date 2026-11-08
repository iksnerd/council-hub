// Package tailnet lets the Go server join a tailnet as its own device (tsnet)
// and serve /mcp and /health there, with the caller's Tailscale identity as the
// access check. It is off by default and configured from the UI.
package tailnet

import "strings"

// Access says who may reach the tailnet listener.
type Access string

const (
	// AccessOwner admits only the Tailscale user who owns this device.
	AccessOwner Access = "owner"
	// AccessMembers admits any identified user or tagged device the tailnet's
	// ACLs let reach this device.
	AccessMembers Access = "members"
)

// Valid reports whether a is a known access mode.
func (a Access) Valid() bool { return a == AccessOwner || a == AccessMembers }

// Identity is who Tailscale says is on the other end of a connection.
type Identity struct {
	LoginName string // empty when Tailscale cannot name a user
	Tagged    bool   // a tagged device has no owner and no login name
}

// Allowed applies the access mode to a caller. owner is the login name of the
// user who owns this device; an empty owner admits nobody in owner mode.
// The default is deny: unknown modes, unidentified callers and tagged devices in
// owner mode are all refused.
func Allowed(mode Access, owner string, id Identity) bool {
	switch mode {
	case AccessOwner:
		return owner != "" && !id.Tagged && strings.EqualFold(id.LoginName, owner)
	case AccessMembers:
		return id.Tagged || id.LoginName != ""
	default:
		return false
	}
}
