package handlers

import (
	"council-hub/internal/council"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// textResult wraps a plain-text response in the standard MCP tool-result tuple
// that every handler returns. Handlers alias it as `msg := textResult` so the
// terse `msg("...")` call sites stay unchanged.
func textResult(text string) (*mcp.CallToolResult, ToolOutput, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, ToolOutput{Message: text}, nil
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

// appendMessageBlock writes one message in the compact "[#id ts] author (type):"
// form shared by get_or_create_room, read_room (include_last_n), and the cluster
// read_room transcript. ts must already be a formatted "2006-01-02 15:04:05"
// string (callers derive it differently — time.Time vs a string field). A plain
// "message" type (or empty) omits the trailing "(type)" tag. {sha:...} tokens in
// the body resolve to commit links using the room's repo (empty repo → bare code
// spans), matching FormatTranscript.
func appendMessageBlock(b *strings.Builder, id, ts, author, msgType, content, repo string) {
	content = council.ResolveCommitRefs(content, repo)
	if msgType != "" && msgType != "message" {
		fmt.Fprintf(b, "\n**[#%.8s %s] %s (%s):**\n%s\n", id, ts, author, msgType, content)
	} else {
		fmt.Fprintf(b, "\n**[#%.8s %s] %s:**\n%s\n", id, ts, author, content)
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
	maxIDLen       = 255
	maxAuthorLen   = 255
	maxContentLen  = 100_000 // ~100KB
	maxMetadataLen = 10_000  // topic, project, tech_stack, tags, system_prompt
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

// Registry holds the council server and handles MCP tool registration.
type Registry struct {
	Server        *council.Server
	HTTPClient    *http.Client // for cluster-wide queries via Phoenix internal API
	PhoenixURL    string       // e.g. "http://127.0.0.1:4000"
	PeerMCPPort   string       // MCP HTTP port used to reach peer Go servers for cross-node writes (default "3001")
	ClusterSecret string       // shared secret (RELEASE_COOKIE) authenticating cross-node write proxies
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
		"type":                 "object",
		"properties":           props,
		"additionalProperties": true,
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
