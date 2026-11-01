package handlers

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"council-hub/internal/council"
)

// internalPinRequest is the body forwarded to a peer node's internal pin endpoint.
// MessageID may be a short prefix: the owner resolves it against its own room.
type internalPinRequest struct {
	RoomID    string `json:"room_id"`
	MessageID string `json:"message_id"`
}

// internalPinResponse is the owner's reply. Pinned is false when the call
// toggled an existing pin off.
type internalPinResponse struct {
	RoomID      string `json:"room_id"`
	MessageID   string `json:"message_id"`
	Pinned      bool   `json:"pinned"`
	ReplacedPin string `json:"replaced_pin,omitempty"`
	Error       string `json:"error"`
}

// errPeerLacksEndpoint marks a 404 from a peer running a release that predates
// an internal endpoint (mixed-version cluster), so callers can say so instead of
// dumping an HTTP error.
var errPeerLacksEndpoint = errors.New("peer runs an older version without this endpoint")

// proxyPinMessage forwards a pin_message toggle to the node that owns the room.
func (r *Registry) proxyPinMessage(owner, roomID, messageID string) (internalPinResponse, error) {
	url, err := r.peerMCPURL(owner, internalPinPath)
	if err != nil {
		return internalPinResponse{}, err
	}
	reqBody, err := json.Marshal(internalPinRequest{RoomID: roomID, MessageID: messageID})
	if err != nil {
		return internalPinResponse{}, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return internalPinResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(clusterSecretHeader, r.ClusterSecret)

	resp, err := r.HTTPClient.Do(httpReq)
	if err != nil {
		return internalPinResponse{}, fmt.Errorf("proxy to %s: %w", owner, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return internalPinResponse{}, errPeerLacksEndpoint
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return internalPinResponse{}, fmt.Errorf("owner node %s returned %d: %s", owner, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var out internalPinResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return internalPinResponse{}, fmt.Errorf("decode owner response: %w", err)
	}
	if out.Error != "" {
		return internalPinResponse{}, fmt.Errorf("owner node %s: %s", owner, out.Error)
	}
	return out, nil
}

// InternalPinHandler receives a cross-node pin toggle proxied from a peer and
// applies it locally. Authenticated by the shared cluster secret. Mounted at
// POST /api/internal/pin_message.
func (r *Registry) InternalPinHandler() http.HandlerFunc {
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
		var in internalPinRequest
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		writeJSON := func(v internalPinResponse) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
		}
		if in.RoomID == "" || in.MessageID == "" {
			writeJSON(internalPinResponse{Error: "room_id and message_id are required"})
			return
		}
		if _, err := r.Server.GetRoom(in.RoomID); err != nil {
			writeJSON(internalPinResponse{Error: fmt.Sprintf("room '%s' not found on owner node", in.RoomID)})
			return
		}
		id := in.MessageID
		if resolved, rerr := r.resolveRef(id, in.RoomID); rerr != nil {
			var amb *council.ErrAmbiguousMessageID
			if errors.As(rerr, &amb) {
				writeJSON(internalPinResponse{Error: rerr.Error()})
				return
			}
		} else {
			id = resolved
		}
		pinned, replaced, err := r.Server.PinMessageReplacing(in.RoomID, id)
		if err != nil {
			writeJSON(internalPinResponse{Error: err.Error()})
			return
		}
		r.Server.Logger.Info("Cross-node pin applied", "room_id", in.RoomID, "msg_id", id, "pinned", pinned)
		writeJSON(internalPinResponse{RoomID: in.RoomID, MessageID: id, Pinned: pinned, ReplacedPin: replaced})
	}
}
