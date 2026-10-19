package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// installValidationHints rewrites the SDK's bare `unexpected additional
// properties ["x"]` rejection so it names the tool's accepted parameters and the
// closest real spelling. The bare form said what was wrong but not what was
// right, so each typo cost a second call to discover the schema.
func (r *Registry) installValidationHints() {
	r.Server.MCP.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			if method != "tools/call" || err != nil {
				return res, err
			}
			call, ok := res.(*mcp.CallToolResult)
			if !ok || call == nil || !call.IsError || len(call.Content) == 0 {
				return res, err
			}
			tc, ok := call.Content[0].(*mcp.TextContent)
			if !ok || !strings.Contains(tc.Text, "unexpected additional properties") {
				return res, err
			}
			cp, ok := req.GetParams().(*mcp.CallToolParamsRaw)
			if !ok || cp == nil {
				return res, err
			}
			listed, lerr := next(ctx, "tools/list", &mcp.ListToolsRequest{Session: req.GetSession().(*mcp.ServerSession), Params: &mcp.ListToolsParams{}})
			if lerr != nil {
				return res, err
			}
			tools, ok := listed.(*mcp.ListToolsResult)
			if !ok {
				return res, err
			}
			for _, tool := range tools.Tools {
				if tool.Name == cp.Name {
					tc.Text += acceptedParamsHint(tool.InputSchema, tc.Text, tool.Name)
				}
			}
			return res, err
		}
	})
}

var quotedRe = regexp.MustCompile(`"([A-Za-z0-9_]+)"`)

func acceptedParamsHint(inputSchema any, errText, tool string) string {
	raw, _ := json.Marshal(inputSchema)
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	accepted := make([]string, 0, len(s.Properties))
	for k := range s.Properties {
		accepted = append(accepted, k)
	}
	sort.Strings(accepted)
	var b strings.Builder
	if i := strings.Index(errText, "additional properties"); i >= 0 {
		for _, m := range quotedRe.FindAllStringSubmatch(errText[i:], -1) {
			if sg := suggestParam(m[1], accepted); sg != "" {
				fmt.Fprintf(&b, " — %q is not a parameter of %s, did you mean %q?", m[1], tool, sg)
			}
		}
	}
	fmt.Fprintf(&b, " (accepted parameters for %s: %s)", tool, strings.Join(accepted, ", "))
	return b.String()
}

// suggestParam returns the accepted name closest to got, or "" if none is close.
// Prefix/substring matches win (include_last -> include_last_n); otherwise an
// edit distance of at most a third of the name.
func suggestParam(got string, accepted []string) string {
	best, bestD := "", 1<<30
	for _, a := range accepted {
		if len(got) >= 4 && strings.Contains(a, got) {
			return a
		}
		if d := editDistance(got, a); d < bestD {
			best, bestD = a, d
		}
	}
	if bestD <= len(got)/3+1 && bestD < len(best) {
		return best
	}
	return ""
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+c)
		}
		prev = cur
	}
	return prev[len(b)]
}
