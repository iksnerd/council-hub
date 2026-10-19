package handlers

import (
	"context"
	"council-hub/internal/council"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// StringBool accepts the historical string form ("true") and JSON booleans.
// MCP tool schemas still advertise strings for compatibility, but some clients
// naturally send true/false for boolean-looking flags; treating those as false
// made cluster_wide reads look empty instead of visibly mis-typed.
type StringBool string

func (b *StringBool) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*b = StringBool(s)
		return nil
	}
	var v bool
	if err := json.Unmarshal(data, &v); err == nil {
		if v {
			*b = "true"
		} else {
			*b = "false"
		}
		return nil
	}
	return fmt.Errorf("expected string or boolean")
}

// textResult wraps a plain-text response in the standard MCP tool-result tuple
// that every handler returns. Handlers alias it as `msg := textResult` so the
// terse `msg("...")` call sites stay unchanged.
func textResult(text string) (*mcp.CallToolResult, ToolOutput, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, ToolOutput{Message: text}, nil
}

// splitIDList splits a comma-separated ID list into trimmed, non-empty IDs.
func splitIDList(s string) []string {
	var ids []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			ids = append(ids, p)
		}
	}
	return ids
}

// resolveIDList resolves each id in ids via ResolveMessageID (exact match, or a
// unique prefix). An id that doesn't resolve is passed through unchanged — this
// covers both "genuinely doesn't exist" (the caller's own not-found reporting,
// e.g. delete_messages/move_messages listing skipped IDs, already handles that
// fine) and an infra failure resolving it (e.g. the DB going away mid-request —
// don't mask that as "not found"; let the real operation attempt against the
// same DB and surface its own accurate error). An *ambiguous* prefix is the one
// case genuinely different from either of those (several real messages match)
// and must not resolve silently to whichever one happens to sort first, so it's
// returned as an error naming the candidates.
func (r *Registry) resolveIDList(ids []string) ([]string, error) {
	resolved := make([]string, len(ids))
	for i, id := range ids {
		got, err := r.Server.ResolveMessageID(id)
		if err != nil {
			if ambiguous, ok := err.(*council.ErrAmbiguousMessageID); ok {
				return nil, ambiguous
			}
			resolved[i] = id
			continue
		}
		resolved[i] = got
	}
	return resolved, nil
}

// resolveSingleID is resolveIDList for the common single-ID case.
func (r *Registry) resolveSingleID(id string) (string, error) {
	resolved, err := r.resolveIDList([]string{id})
	if err != nil {
		return "", err
	}
	return resolved[0], nil
}

// resolveInto resolves *id in place via resolveSingleID (exact match, or a
// unique prefix — an unresolvable-but-unambiguous id is left unchanged so the
// caller's own not-found handling still applies). Collapses the
// "resolved, err := r.resolveSingleID(id); if err != nil {...}; id = resolved"
// shape repeated at every single-ID call site down to one line.
func (r *Registry) resolveInto(id *string) error {
	resolved, err := r.resolveSingleID(*id)
	if err != nil {
		return err
	}
	*id = resolved
	return nil
}

// appendMessageBlock writes one message in the compact "[#id ts] author (type):"
// form shared by get_or_create_room, read_room (include_last_n), and the cluster
// read_room transcript. ts must already be a formatted "2006-01-02 15:04:05"
// string (callers derive it differently — time.Time vs a string field). A plain
// "message" type (or empty) omits the trailing "(type)" tag. {sha:...} tokens in
// the body resolve to commit links using the room's repo (empty repo → bare code
// spans), matching FormatTranscript.
func appendMessageBlock(b *strings.Builder, ids *council.ShortIDs, id, ts, author, msgType, content, repo, workItems string) {
	content = council.ResolveRefs(content, repo, workItems)
	if msgType != "" && msgType != "message" {
		fmt.Fprintf(b, "\n**[#%s %s] %s (%s):**\n%s\n", ids.Of(id), ts, author, msgType, content)
	} else {
		fmt.Fprintf(b, "\n**[#%s %s] %s:**\n%s\n", ids.Of(id), ts, author, content)
	}
}

// healthTagActions maps each Knowledge-Linter health flag to the corrective move
// that clears it, in priority order. Surfaced next to a room so the flag is
// actionable, not just visible — agents reported the flags read as dead-ends.
var healthTagActions = []struct{ tag, action string }{
	{"incoherent", "a `contradicts`/`duplicates` edge has no reconciling synthesis — resolve the conflict, then post a `synthesis` (clears on synthesis or a supersedes link)"},
	{"needs-synthesis", "decisions/actions accumulated with no synthesis — distill them into a `synthesis` and pin it (post_to_room with pin=true)"},
	{"stale-pin", "the pinned synthesis is outdated — post a fresh `synthesis` and pin it (post_to_room with pin=true) to replace it"},
	{"stale-plan", "a `plan` was posted but never executed — post an `action` that carries it out, or close the room"},
	{"unpinned-synthesis", "a synthesis exists but is not pinned — pin it so the room has a living TL;DR"},
	{"stale", "inactive — resume it, post a `synthesis` and `signal_status(resolved)`, or archive it"},
}

// healthTagHint returns a one-line "→ what to do" block for any health flags
// present on a room's tags, or "" if there are none. The leading newline lets
// callers append it directly to a response body.
func healthTagHint(tags string) string {
	var lines []string
	for _, ht := range healthTagActions {
		for _, t := range strings.Split(tags, ",") {
			if strings.TrimSpace(t) == ht.tag {
				lines = append(lines, fmt.Sprintf("  → `%s`: %s", ht.tag, ht.action))
				break
			}
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n**Health flags — suggested actions:**\n" + strings.Join(lines, "\n") + "\n"
}

// Input size limits to prevent DoS and unbounded database growth.
const (
	maxIDLen        = 255
	maxAuthorLen    = 255
	maxContentLen   = 100_000 // ~100KB
	maxMetadataLen  = 10_000  // topic, project, tech_stack, tags, system_prompt
	maxWorkspaceLen = 1024    // working-tree path; PATH_MAX on the platforms clients run on
)

// validateSize returns an error if value exceeds max characters.
func validateSize(field, value string, max int) error {
	if len(value) > max {
		return fmt.Errorf("%s exceeds maximum length (%d chars, limit %d)", field, len(value), max)
	}
	return nil
}

// validateRoomMetadata checks size limits on all room metadata fields.
func validateRoomMetadata(topic, project, techStack, tags, systemPrompt string) error {
	for _, check := range []struct{ name, val string }{
		{"topic", topic}, {"project", project}, {"tech_stack", techStack},
		{"tags", tags}, {"system_prompt", systemPrompt},
	} {
		if err := validateSize(check.name, check.val, maxMetadataLen); err != nil {
			return err
		}
	}
	return nil
}

// maxWorkItemsLen caps a work_items URL template.
const maxWorkItemsLen = 500

// validateWorkItems checks a room's work_items URL template: empty is allowed
// (no template), otherwise it must be an http(s) URL containing the {id}
// placeholder. The scheme check keeps javascript:/data: URLs out of the
// markdown links {wi:...} renders.
func validateWorkItems(tmpl string) error {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		return nil
	}
	if err := validateSize("work_items", tmpl, maxWorkItemsLen); err != nil {
		return err
	}
	if !strings.HasPrefix(tmpl, "http://") && !strings.HasPrefix(tmpl, "https://") {
		return fmt.Errorf("work_items must start with http:// or https:// (e.g. https://tracker.example.com/items/{id})")
	}
	if !strings.Contains(tmpl, "{id}") {
		return fmt.Errorf("work_items must contain the {id} placeholder (e.g. https://tracker.example.com/items/{id})")
	}
	return nil
}

// Registry holds the council server and handles MCP tool registration.
type Registry struct {
	Server        *council.Server
	HTTPClient    *http.Client // for cluster-wide queries via Phoenix internal API
	PhoenixURL    string       // e.g. "http://127.0.0.1:4000"
	PeerMCPPort   string       // MCP HTTP port used to reach peer Go servers for cross-node writes (default "3001")
	ClusterSecret string       // shared secret (RELEASE_COOKIE) authenticating cross-node write proxies

	nodeMu     sync.Mutex // guards the cached cluster node count (see clusterNodeCount)
	nodeAt     time.Time
	nodeN      int
	nodeFailed bool // last lookup failed; cached for nodeCountFailTTL
}

// toolResultText extracts the text content from a CallToolResult.
func toolResultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	if tc, ok := r.Content[0].(*mcp.TextContent); ok && tc != nil {
		return tc.Text
	}
	return ""
}

// ToolOutput is the structured output type for tool results.
type ToolOutput struct {
	Message string `json:"message"`
}

// plainTool adapts a text-only handler so the SDK neither derives an
// outputSchema nor emits structuredContent for it. Registering such a handler
// with ToolOutput advertised a {"message": string} schema and echoed the
// markdown a second time as structuredContent; the Content text is the real
// result. Tools with real structured output (list_rooms) register directly.
func plainTool[In any](h func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, ToolOutput, error)) func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		res, _, err := h(ctx, req, in)
		return res, nil, err
	}
}

// ListedRoom is one row of list_rooms' structured output, built from the same
// data the markdown renderer prints.
type ListedRoom struct {
	ID            string   `json:"id"`
	Topic         string   `json:"topic"`
	Status        string   `json:"status"`
	Project       string   `json:"project"`
	Tags          []string `json:"tags"`
	RelatedRooms  []string `json:"related_rooms"`
	PinnedExcerpt string   `json:"pinned_excerpt,omitempty"`
	SourceNode    string   `json:"source_node,omitempty"`
	Updated       string   `json:"updated"`
}

// ListRoomsOutput is list_rooms' structured output. OmittedResolved counts the
// resolved rooms the default view hid (on a cluster read, the distinct resolved
// rooms across reachable nodes). Pinned excerpts are local-only: peers do not send them.
type ListRoomsOutput struct {
	Rooms           []ListedRoom `json:"rooms"`
	Warnings        []string     `json:"warnings"`
	OmittedResolved int          `json:"omitted_resolved"`
}

// newListRoomsOutput guarantees non-nil slices so they serialize as [] and
// satisfy the generated output schema.
func newListRoomsOutput(rooms []ListedRoom, warnings []string) ListRoomsOutput {
	if rooms == nil {
		rooms = []ListedRoom{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	return ListRoomsOutput{Rooms: rooms, Warnings: warnings}
}

func listRoomsResult(text string, out ListRoomsOutput) (*mcp.CallToolResult, ListRoomsOutput, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, out, nil
}

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func listedRoomFromCluster(rm ClusterRoomResult) ListedRoom {
	return ListedRoom{
		ID:           rm.ID,
		Topic:        rm.Description,
		Status:       rm.Status,
		Project:      rm.Project,
		Tags:         splitCSV(rm.Tags),
		RelatedRooms: splitCSV(rm.RelatedRooms),
		SourceNode:   rm.SourceNode,
		Updated:      rm.UpdatedAt,
	}
}

// validRoomStatuses gates every path that writes rooms.status — the signal_status
// tool, bulk_status_update, and the cross-node InternalStatusHandler (which must
// validate independently, since a peer supplies its input).
var validRoomStatuses = map[string]bool{
	"active":   true,
	"paused":   true,
	"resolved": true,
}

var validMessageTypes = map[string]bool{
	"message":   true,
	"thought":   true,
	"draft":     true,
	"decision":  true,
	"review":    true,
	"action":    true,
	"critique":  true,
	"synthesis": true,
	"note":      true,
	"plan":      true,
}

// schema builds a JSON Schema object with additionalProperties: true.
// required lists the field names that are mandatory; all others are optional.
func schema(required []string, props map[string]map[string]any) map[string]any {
	s := map[string]any{
		"type":       "object",
		"properties": props,
		// Unknown properties are rejected rather than dropped. A dropped typo
		// leaves a required field at "", and the failure then surfaces as a
		// downstream symptom — "room '' not found" for a room the caller had
		// just created — which cost three separate sessions several calls each
		// before anyone recognised it as a misspelled parameter (#01a0b156).
		// The validator names the offending property instead.
		"additionalProperties": false,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func prop(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

func enumProp(typ, desc string, enum []string) map[string]any {
	return map[string]any{"type": typ, "description": desc, "enum": enum}
}

// resolveAlias picks between two accepted spellings of one parameter. Supplying
// both is an error rather than a silent preference: the two values can differ,
// and quietly dropping one is the failure mode that made unknown-parameter typos
// so expensive to diagnose in the first place.
func resolveAlias(canonical, canonicalName, alias, aliasName string) (string, error) {
	switch {
	case canonical != "" && alias != "" && canonical != alias:
		return "", fmt.Errorf("%s and %s are two spellings of the same parameter and were given different values (%q vs %q) — pass one", canonicalName, aliasName, canonical, alias)
	case canonical != "":
		return canonical, nil
	default:
		return alias, nil
	}
}

// clientIdentity is the name the connected MCP client gave at initialize, used
// as the default author. It is the only identity the server actually knows, and
// it removes the one rejection in the parameter set that costs a re-sent body.
func clientIdentity(req *mcp.CallToolRequest) string {
	if req == nil || req.Session == nil {
		return ""
	}
	params := req.Session.InitializeParams()
	if params == nil || params.ClientInfo == nil {
		return ""
	}
	return strings.TrimSpace(params.ClientInfo.Name)
}

// roomIDCache hands out per-room ShortIDs for output that spans rooms (the
// notebook timeline, outlines). One query per room, memoized for the render. A
// nil cache, or a room it cannot load, prints the plain 8-char prefix.
type roomIDCache struct {
	server *council.Server
	rooms  map[string]*council.ShortIDs
}

func newRoomIDCache(s *council.Server) *roomIDCache {
	return &roomIDCache{server: s, rooms: map[string]*council.ShortIDs{}}
}

// Of returns the display prefix for message id within roomID.
func (c *roomIDCache) Of(roomID, id string) string {
	if c == nil || c.server == nil {
		return (*council.ShortIDs)(nil).Of(id)
	}
	ids, ok := c.rooms[roomID]
	if !ok {
		ids = c.server.ShortIDsOrDefault(roomID)
		c.rooms[roomID] = ids
	}
	return ids.Of(id)
}
