package handlers

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"council-hub/internal/council"
)

// clusterSecretHeader authenticates cross-node write proxies. The value is the
// shared RELEASE_COOKIE, which is already required for Erlang clustering, so no
// new secret needs provisioning.
const clusterSecretHeader = "X-Council-Cluster-Secret"

// Internal cross-node endpoint paths, shared by the proxy helpers here and the
// route table in main.go.
const (
	internalPostPath   = "/api/internal/post_to_room"
	internalStatusPath = "/api/internal/signal_status"
)

// internalPostRequest is the body forwarded to a peer node's internal write endpoint.
type internalPostRequest struct {
	RoomID      string `json:"room_id"`
	Author      string `json:"author"`
	Message     string `json:"message"`
	MessageType string `json:"message_type"`
	ReplyTo     string `json:"reply_to"`
	Mentions    string `json:"mentions"`
	Supersedes  string `json:"supersedes"`
	Pin         string `json:"pin"`
}

// internalStatusRequest is the body forwarded to a peer node's internal status endpoint.
type internalStatusRequest struct {
	RoomID string `json:"room_id"`
	Status string `json:"status"`
}

// internalStatusResponse is the peer node's reply after applying the status change.
type internalStatusResponse struct {
	RoomID string `json:"room_id"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

// internalPostResponse is the peer node's reply after writing the message locally.
type internalPostResponse struct {
	MessageID string `json:"message_id"`
	RoomID    string `json:"room_id"`
	// Pinned reports whether a requested pin actually landed on the owner. A
	// failed pin doesn't fail the write (the message is already stored), so the
	// caller needs this to avoid claiming a pin that didn't happen.
	Pinned bool   `json:"pinned"`
	Error  string `json:"error"`
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

// peerMCPURL turns a node name like "council_hub@10.0.0.5" plus an internal
// endpoint path into that node's URL, e.g.
// "http://10.0.0.5:3001/api/internal/post_to_room".
func (r *Registry) peerMCPURL(node, path string) (string, error) {
	at := strings.LastIndex(node, "@")
	if at < 0 || at == len(node)-1 {
		return "", fmt.Errorf("cannot derive host from node name %q", node)
	}
	host := node[at+1:]
	port := r.PeerMCPPort
	if port == "" {
		port = "3001"
	}
	return fmt.Sprintf("http://%s:%s%s", host, port, path), nil
}

// proxyPostToRoom forwards a post_to_room write to the node that owns the room.
// Returns the new message ID and whether a requested pin landed there.
func (r *Registry) proxyPostToRoom(owner string, args PostToRoomInput) (string, bool, error) {
	url, err := r.peerMCPURL(owner, internalPostPath)
	if err != nil {
		return "", false, err
	}

	reqBody, err := json.Marshal(internalPostRequest{
		RoomID:      args.RoomID,
		Author:      args.Author,
		Message:     args.Message,
		MessageType: args.MessageType,
		ReplyTo:     args.ReplyTo,
		Mentions:    args.Mentions,
		Supersedes:  args.Supersedes,
		Pin:         args.Pin,
	})
	if err != nil {
		return "", false, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(clusterSecretHeader, r.ClusterSecret)

	resp, err := r.HTTPClient.Do(httpReq)
	if err != nil {
		return "", false, fmt.Errorf("proxy to %s: %w", owner, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("owner node %s returned %d: %s", owner, resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out internalPostResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", false, fmt.Errorf("decode owner response: %w", err)
	}
	if out.Error != "" {
		return "", false, fmt.Errorf("owner node %s: %s", owner, out.Error)
	}
	return out.MessageID, out.Pinned, nil
}

// Remedies for remoteRoomNote, matching what the calling tool can actually offer.
const (
	remedyClusterRead = "retry with cluster_wide=true"
	remedyLocalOnly   = "this tool is local-only — run it from a session on that node"
)

// remoteRoomNote turns "room not found" into the truth when a cluster peer owns
// the room. Left un-annotated, that error is indistinguishable from "this room
// does not exist", so the reasonable next move is to create it — producing
// exactly the local shadow room that create_room's conflict guard exists to
// prevent. Returns "" when no peer owns it (genuinely missing) or the cluster
// isn't configured, so non-clustered deployments see no change.
//
// Costs one locate_room round trip, so call it only on the error path, and
// never in a loop over many room IDs.
func (r *Registry) remoteRoomNote(roomID, remedy string) string {
	owner, err := r.locateRoomOwner(roomID)
	if err != nil || owner == "" {
		return ""
	}
	return fmt.Sprintf(" It lives on cluster node '%s' — %s.", owner, remedy)
}

// proxyStatusUpdate forwards a signal_status change to the node that owns the room.
// Status is a room-metadata write, so it must land on the owner the same way a
// message does — otherwise a room can be closed out from any node (post_to_room
// already proxies) while its status flip silently 404s, leaving the Knowledge
// Linter flagging a room whose work is finished and recorded.
func (r *Registry) proxyStatusUpdate(owner, roomID, status string) error {
	url, err := r.peerMCPURL(owner, internalStatusPath)
	if err != nil {
		return err
	}

	reqBody, err := json.Marshal(internalStatusRequest{RoomID: roomID, Status: status})
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(clusterSecretHeader, r.ClusterSecret)

	resp, err := r.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("proxy to %s: %w", owner, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("owner node %s returned %d: %s", owner, resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out internalStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("decode owner response: %w", err)
	}
	if out.Error != "" {
		return fmt.Errorf("owner node %s: %s", owner, out.Error)
	}
	return nil
}

// InternalStatusHandler receives a cross-node status change proxied from a peer
// Go server and applies it locally. Authenticated by the shared cluster secret,
// exactly like InternalPostHandler. Mounted at POST /api/internal/signal_status.
func (r *Registry) InternalStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		got := req.Header.Get(clusterSecretHeader)
		if r.ClusterSecret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(r.ClusterSecret)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		var in internalStatusRequest
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		writeJSON := func(v internalStatusResponse) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
		}

		if in.RoomID == "" || in.Status == "" {
			writeJSON(internalStatusResponse{Error: "room_id and status are required"})
			return
		}
		// Re-validate here rather than trusting the calling node: this endpoint is
		// reachable by any peer holding the cluster secret, not only by our own
		// signal_status handler.
		if !validRoomStatuses[in.Status] {
			writeJSON(internalStatusResponse{Error: fmt.Sprintf("invalid status '%s'", in.Status)})
			return
		}

		if err := r.Server.UpdateStatus(in.RoomID, in.Status); err != nil {
			r.Server.Logger.Error("Cross-node status change failed", "room_id", in.RoomID, "error", err)
			writeJSON(internalStatusResponse{Error: err.Error()})
			return
		}

		r.Server.Logger.Info("Cross-node status change applied", "room_id", in.RoomID, "status", in.Status)
		writeJSON(internalStatusResponse{RoomID: in.RoomID, Status: in.Status})
	}
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

		// Apply pin here rather than dropping it: the close-out workflow is
		// post synthesis -> pin -> resolve, and a pin that silently vanished on
		// the proxy path left peer-owned rooms permanently flagged stale-pin with
		// no way to fix them from another node.
		pinned := false
		if in.Pin == "true" {
			if _, perr := r.Server.PinMessage(in.RoomID, msgID); perr != nil {
				// The write already succeeded — report the partial outcome instead
				// of failing the whole call.
				r.Server.Logger.Warn("Cross-node pin failed", "room_id", in.RoomID, "msg_id", msgID, "error", perr)
			} else {
				pinned = true
			}
		}

		r.Server.Logger.Info("Cross-node write applied", "room_id", in.RoomID, "author", in.Author, "msg_id", msgID, "pinned", pinned)
		writeJSON(internalPostResponse{MessageID: msgID, RoomID: in.RoomID, Pinned: pinned})
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
			// A duplicate ref is a benign no-op: return the pre-existing entry, not an error.
			var dup *council.ErrAlreadyReferenced
			if errors.As(err, &dup) {
				writeJSON(uiNotebookEntryResponse{EntryID: dup.EntryID, NotebookID: in.NotebookID})
				return
			}
			r.Server.Logger.Error("UI notebook entry failed", "notebook_id", in.NotebookID, "error", err)
			writeJSON(uiNotebookEntryResponse{Error: err.Error()})
			return
		}

		r.Server.Logger.Info("UI notebook entry added", "notebook_id", in.NotebookID, "entry_id", entryID, "kind", kind)
		writeJSON(uiNotebookEntryResponse{EntryID: entryID, NotebookID: in.NotebookID})
	}
}

// uiBackfillEmbeddingsRequest is the payload for the /status "regenerate" button.
type uiBackfillEmbeddingsRequest struct {
	Full bool `json:"full"`
}

type uiBackfillEmbeddingsResponse struct {
	Started     bool   `json:"started"`
	Full        bool   `json:"full"`
	MsgTotal    int    `json:"msg_total"`
	MsgIndexed  int    `json:"msg_indexed"`
	RoomTotal   int    `json:"room_total"`
	RoomIndexed int    `json:"room_indexed"`
	Error       string `json:"error,omitempty"`
}

// UIBackfillEmbeddingsHandler lets the Phoenix dashboard's /status page
// trigger an on-demand embedding backfill (or, with full=true, a full
// clear-and-recompute) without going through an MCP session. Same trust
// model as the other /api/ui/* endpoints: localhost-only. Coverage counts
// in the response are a "before" snapshot — the job itself runs in the
// background and reports completion only via server logs / GET /health.
// Mounted at POST /api/ui/backfill_embeddings.
func (r *Registry) UIBackfillEmbeddingsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireLocalhostPost(w, req) {
			return
		}

		writeJSON := func(v uiBackfillEmbeddingsResponse) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
		}

		if r.Server.Embedder == nil {
			writeJSON(uiBackfillEmbeddingsResponse{Error: "semantic search is not enabled (COUNCIL_OLLAMA_URL not set)"})
			return
		}

		var in uiBackfillEmbeddingsRequest
		_ = json.NewDecoder(req.Body).Decode(&in) // an empty/absent body just means full=false

		msgTotal, msgIndexed, roomTotal, roomIndexed := r.Server.EmbeddingCoverage()
		// A context tied to this request would be canceled the moment the
		// handler returns; the job runs in the background well past that.
		started := r.Server.TriggerEmbedJob(context.Background(), in.Full)

		r.Server.Logger.Info("UI embedding regeneration triggered", "full", in.Full, "started", started)
		writeJSON(uiBackfillEmbeddingsResponse{
			Started: started, Full: in.Full,
			MsgTotal: msgTotal, MsgIndexed: msgIndexed,
			RoomTotal: roomTotal, RoomIndexed: roomIndexed,
		})
	}
}
