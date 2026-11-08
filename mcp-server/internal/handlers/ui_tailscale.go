package handlers

import (
	"encoding/json"
	"net/http"

	"council-hub/internal/tailnet"
)

// uiTailscaleRequest is the body of POST /api/ui/tailscale. A field left out
// keeps its saved value (the defaults on a first run), so {"enabled":true} is a
// complete "sign in" request. AuthKey is used for one headless sign-in and is
// neither stored nor echoed back.
type uiTailscaleRequest struct {
	Enabled    bool            `json:"enabled"`
	Hostname   *string         `json:"hostname"`
	Access     *tailnet.Access `json:"access"`
	ControlURL *string         `json:"control_url"`
	Tags       *[]string       `json:"tags"`
	AuthKey    string          `json:"auth_key"`
	// Action "logout" forgets the Tailscale login and turns it off (the device
	// stays listed, offline, in the Tailscale admin console); every other field
	// is then ignored. Empty means apply the config.
	Action string `json:"action"`
}

// UITailscaleHandler lets the Phoenix settings page read and change the
// built-in Tailscale node. GET returns the status; POST applies a new config.
// Localhost only, like the other /api/ui routes, and never mounted on the
// tailnet listener.
func (r *Registry) UITailscaleHandler(m *tailnet.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireLocalhost(w, req) {
			return
		}
		switch req.Method {
		case http.MethodGet:
		case http.MethodPost:
			var in uiTailscaleRequest
			if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<16)).Decode(&in); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			if in.Action == "logout" {
				if err := m.Logout(req.Context()); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				r.Server.Logger.Info("UI signed the tailscale device out")
				break
			}
			if in.Action != "" {
				http.Error(w, "unknown action", http.StatusBadRequest)
				return
			}
			cfg := m.Status().Config
			cfg.Enabled = in.Enabled
			if in.Hostname != nil {
				cfg.Hostname = *in.Hostname
			}
			if in.Access != nil {
				cfg.Access = *in.Access
			}
			if in.ControlURL != nil {
				cfg.ControlURL = *in.ControlURL
			}
			if in.Tags != nil {
				cfg.Tags = *in.Tags
			}
			if err := m.Apply(cfg, in.AuthKey); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			r.Server.Logger.Info("UI changed tailscale settings", "enabled", cfg.Enabled, "access", cfg.Access)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.Status())
	}
}
