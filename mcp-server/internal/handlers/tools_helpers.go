package handlers

import (
	"council-hub/internal/council"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	Server     *council.Server
	HTTPClient *http.Client // for cluster-wide queries via Phoenix internal API
	PhoenixURL string       // e.g. "http://127.0.0.1:4000"
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
	"code":      true,
	"review":    true,
	"action":    true,
	"critique":  true,
	"synthesis": true,
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
