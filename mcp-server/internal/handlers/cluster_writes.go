package handlers

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// clusterSecretHeader authenticates cross-node write proxies. The value is the
// shared RELEASE_COOKIE, which is already required for Erlang clustering, so no
// new secret needs provisioning.
const clusterSecretHeader = "X-Council-Cluster-Secret"

// internalPostRequest is the body forwarded to a peer node's internal write endpoint.
type internalPostRequest struct {
	RoomID      string `json:"room_id"`
	Author      string `json:"author"`
	Message     string `json:"message"`
	MessageType string `json:"message_type"`
	ReplyTo     string `json:"reply_to"`
	Mentions    string `json:"mentions"`
	Supersedes  string `json:"supersedes"`
}

// internalPostResponse is the peer node's reply after writing the message locally.
type internalPostResponse struct {
	MessageID string `json:"message_id"`
	RoomID    string `json:"room_id"`
	Error     string `json:"error"`
}

// locateRoomOwner asks Phoenix which cluster node owns a (public) room. Returns
// the owning node name, or "" if no node owns it / cluster is not configured.
// Private rooms are never reported (the Phoenix fan-out gate excludes them).
func (r *Registry) locateRoomOwner(roomID string) (string, error) {
	if r.HTTPClient == nil || r.PhoenixURL == "" {
		return "", nil
	}

	body, err := json.Marshal(map[string]any{"room_id": roomID})
	if err != nil {
		return "", err
	}

	url := r.PhoenixURL + "/api/internal/cluster/locate_room"
	resp, err := r.HTTPClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("locate_room call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("locate_room returned %d: %s", resp.StatusCode, string(msg))
	}

	var raw struct {
		Nodes    []string `json:"nodes"`
		Warnings []string `json:"warnings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", fmt.Errorf("decode locate_room: %w", err)
	}

	// post_to_room only locates when the room is missing locally, so the owner is
	// never this node — the first reported node is the remote owner.
	if len(raw.Nodes) > 0 {
		return raw.Nodes[0], nil
	}
	return "", nil
}

// peerMCPURL turns a node name like "council_hub@10.0.0.5" into the URL of that
// node's internal write endpoint, e.g. "http://10.0.0.5:3001/api/internal/post_to_room".
func (r *Registry) peerMCPURL(node string) (string, error) {
	at := strings.LastIndex(node, "@")
	if at < 0 || at == len(node)-1 {
		return "", fmt.Errorf("cannot derive host from node name %q", node)
	}
	host := node[at+1:]
	port := r.PeerMCPPort
	if port == "" {
		port = "3001"
	}
	return fmt.Sprintf("http://%s:%s/api/internal/post_to_room", host, port), nil
}

// proxyPostToRoom forwards a post_to_room write to the node that owns the room.
func (r *Registry) proxyPostToRoom(owner string, args PostToRoomInput) (string, error) {
	url, err := r.peerMCPURL(owner)
	if err != nil {
		return "", err
	}

	reqBody, err := json.Marshal(internalPostRequest{
		RoomID:      args.RoomID,
		Author:      args.Author,
		Message:     args.Message,
		MessageType: args.MessageType,
		ReplyTo:     args.ReplyTo,
		Mentions:    args.Mentions,
		Supersedes:  args.Supersedes,
	})
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(clusterSecretHeader, r.ClusterSecret)

	resp, err := r.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("proxy to %s: %w", owner, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("owner node %s returned %d: %s", owner, resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out internalPostResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode owner response: %w", err)
	}
	if out.Error != "" {
		return "", fmt.Errorf("owner node %s: %s", owner, out.Error)
	}
	return out.MessageID, nil
}

// InternalPostHandler receives a cross-node write proxied from a peer Go server
// and applies it to the local database. Authenticated by the shared cluster
// secret. Mounted at POST /api/internal/post_to_room.
func (r *Registry) InternalPostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Cross-node writes require a shared secret. Reject if unset or mismatched.
		// Constant-time compare avoids leaking the secret via response timing.
		got := req.Header.Get(clusterSecretHeader)
		if r.ClusterSecret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(r.ClusterSecret)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		var in internalPostRequest
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		writeJSON := func(v internalPostResponse) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
		}

		if in.RoomID == "" || in.Author == "" || in.Message == "" {
			writeJSON(internalPostResponse{Error: "room_id, author, and message are required"})
			return
		}

		// The room must exist locally on this (owner) node.
		if _, err := r.Server.GetRoom(in.RoomID); err != nil {
			writeJSON(internalPostResponse{Error: fmt.Sprintf("room '%s' not found on owner node", in.RoomID)})
			return
		}

		msgType := in.MessageType
		if msgType == "" {
			msgType = "message"
		}

		msgID, err := r.Server.PostMessageWithRefs(in.RoomID, in.Author, in.Message, msgType, in.ReplyTo, in.Mentions, in.Supersedes)
		if err != nil {
			r.Server.Logger.Error("Cross-node write failed", "room_id", in.RoomID, "error", err)
			writeJSON(internalPostResponse{Error: err.Error()})
			return
		}

		r.Server.Logger.Info("Cross-node write applied", "room_id", in.RoomID, "author", in.Author, "msg_id", msgID)
		writeJSON(internalPostResponse{MessageID: msgID, RoomID: in.RoomID})
	}
}

// requireLocalhostPost gates the /api/ui/* endpoints: POST only, loopback only
// (they exist for the co-located Phoenix UI, which has no MCP session).
// Returns false after writing the error response when the request is rejected.
func requireLocalhostPost(w http.ResponseWriter, req *http.Request) bool {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	host := req.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	host = strings.Trim(host, "[]")
	if host != "127.0.0.1" && host != "::1" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// UIPostHandler allows the Phoenix web UI to post messages without going through
// the MCP protocol (which requires a session handshake). Restricted to localhost.
// Mounted at POST /api/ui/post.
func (r *Registry) UIPostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireLocalhostPost(w, req) {
			return
		}

		var in internalPostRequest
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		writeJSON := func(v internalPostResponse) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
		}

		if in.RoomID == "" || in.Author == "" || in.Message == "" {
			writeJSON(internalPostResponse{Error: "room_id, author, and message are required"})
			return
		}

		msgType := in.MessageType
		if msgType == "" {
			msgType = "message"
		}

		msgID, err := r.Server.PostMessageWithRefs(in.RoomID, in.Author, in.Message, msgType, in.ReplyTo, in.Mentions, in.Supersedes)
		if err != nil {
			r.Server.Logger.Error("UI post failed", "room_id", in.RoomID, "error", err)
			writeJSON(internalPostResponse{Error: err.Error()})
			return
		}

		r.Server.Logger.Info("UI post applied", "room_id", in.RoomID, "author", in.Author, "msg_id", msgID)
		writeJSON(internalPostResponse{MessageID: msgID, RoomID: in.RoomID})
	}
}

// uiNotebookEntryRequest is the payload for the UI "pin into notebook" path.
type uiNotebookEntryRequest struct {
	NotebookID   string `json:"notebook_id"`
	RefID        string `json:"ref_id"`
	Prose        string `json:"prose"`
	AfterEntryID string `json:"after_entry_id"`
}

type uiNotebookEntryResponse struct {
	EntryID    string `json:"entry_id,omitempty"`
	NotebookID string `json:"notebook_id,omitempty"`
	Error      string `json:"error,omitempty"`
}

// UINotebookEntryHandler lets the Phoenix dashboard add an outline entry —
// the "📌 into notebook" button on timeline entries. Same trust model as
// UIPostHandler: localhost-only, no MCP session. Kind is inferred (ref_id →
// ref, prose → prose), matching edit_notebook(action=add).
// Mounted at POST /api/ui/notebook_entry.
func (r *Registry) UINotebookEntryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireLocalhostPost(w, req) {
			return
		}

		var in uiNotebookEntryRequest
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		writeJSON := func(v uiNotebookEntryResponse) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
		}

		if in.NotebookID == "" {
			writeJSON(uiNotebookEntryResponse{Error: "notebook_id is required"})
			return
		}

		kind := "prose"
		if in.RefID != "" {
			kind = "ref"
		}

		entryID, err := r.Server.AddOutlineEntry(in.NotebookID, kind, in.RefID, in.Prose, in.AfterEntryID)
		if err != nil {
			r.Server.Logger.Error("UI notebook entry failed", "notebook_id", in.NotebookID, "error", err)
			writeJSON(uiNotebookEntryResponse{Error: err.Error()})
			return
		}

		r.Server.Logger.Info("UI notebook entry added", "notebook_id", in.NotebookID, "entry_id", entryID, "kind", kind)
		writeJSON(uiNotebookEntryResponse{EntryID: entryID, NotebookID: in.NotebookID})
	}
}
