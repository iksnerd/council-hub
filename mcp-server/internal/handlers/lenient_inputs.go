package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Typed schemas, tolerant servers.
//
// The tools advertise real types (integer, boolean, array of strings), as
// drenskicorp RFC 0013 section 4 asks. The handlers keep their string fields, and
// two small pieces bridge the gap so no client breaks:
//
//   - coerceLegacyArguments (middleware, before validation) turns the old string
//     spellings into the typed values the schema now declares: "true" -> true,
//     "10" -> 10, "a,b" or `["a","b"]` -> ["a","b"]. A value that cannot be
//     converted is left alone, so the validator reports it by name.
//   - Lenient (at decode time) turns the typed values back into the strings the
//     handlers read, so a handler never learns which spelling arrived.

// Lenient decodes a tool's arguments into In, accepting a JSON boolean, number or
// array of strings wherever In has a string field.
type Lenient[In any] struct{ V In }

func (l *Lenient[In]) UnmarshalJSON(data []byte) error {
	norm, err := stringifyForStructFields(data, reflect.TypeOf(l.V))
	if err != nil {
		return err
	}
	return json.Unmarshal(norm, &l.V)
}

var stringFieldCache sync.Map // reflect.Type -> map[string]bool (json name -> field is a string kind)

func stringFields(t reflect.Type) map[string]bool {
	if cached, ok := stringFieldCache.Load(t); ok {
		return cached.(map[string]bool)
	}
	out := map[string]bool{}
	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				continue
			}
			out[name] = f.Type.Kind() == reflect.String
		}
	}
	stringFieldCache.Store(t, out)
	return out
}

// stringifyForStructFields rewrites each non-string JSON value that targets a
// string field into the string the handlers have always read.
func stringifyForStructFields(data []byte, t reflect.Type) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return data, nil // not an object: let the normal decode report it
	}
	isString := stringFields(t)
	for k, v := range raw {
		if !isString[k] {
			continue
		}
		v = bytes.TrimSpace(v)
		switch {
		case len(v) == 0 || v[0] == '"':
		case bytes.Equal(v, []byte("null")):
			delete(raw, k)
		case bytes.Equal(v, []byte("true")), bytes.Equal(v, []byte("false")):
			raw[k] = mustMarshalString(string(v))
		case v[0] == '-' || (v[0] >= '0' && v[0] <= '9'):
			raw[k] = mustMarshalString(string(v))
		case v[0] == '[':
			var items []json.RawMessage
			if err := json.Unmarshal(v, &items); err != nil {
				continue
			}
			parts := make([]string, 0, len(items))
			for _, it := range items {
				var s string
				if json.Unmarshal(it, &s) != nil {
					s = strings.Trim(string(it), `"`)
				}
				if s = strings.TrimSpace(s); s != "" {
					parts = append(parts, s)
				}
			}
			raw[k] = mustMarshalString(strings.Join(parts, ","))
		}
	}
	return json.Marshal(raw)
}

func mustMarshalString(s string) json.RawMessage { b, _ := json.Marshal(s); return b }

// plainTool adapts a text-only handler so the SDK neither derives an
// outputSchema nor emits structuredContent for it. Registering such a handler
// with ToolOutput advertised a {"message": string} schema and echoed the
// markdown a second time as structuredContent; the Content text is the real
// result. Tools with real structured output (list_rooms) register directly.
// Its input is decoded leniently (see Lenient).
func plainTool[In any](h func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, ToolOutput, error)) func(context.Context, *mcp.CallToolRequest, Lenient[In]) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in Lenient[In]) (*mcp.CallToolResult, any, error) {
		res, _, err := h(ctx, req, in.V)
		return res, nil, err
	}
}

// lenientTool is plainTool for a handler with real structured output.
func lenientTool[In, Out any](h func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)) func(context.Context, *mcp.CallToolRequest, Lenient[In]) (*mcp.CallToolResult, Out, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in Lenient[In]) (*mcp.CallToolResult, Out, error) {
		return h(ctx, req, in.V)
	}
}

var intLiteral = regexp.MustCompile(`^-?\d+$`)

// propSpec is what the coercion middleware needs to know about one property:
// its schema type and, for an integer, the advertised range.
type propSpec struct {
	Type     string
	Min, Max *int64
}

// clampInt pins n into the property's advertised range. The handlers have always
// clamped, so an out-of-range value keeps working rather than failing validation.
func (p propSpec) clampInt(n int64) int64 {
	if p.Min != nil && n < *p.Min {
		n = *p.Min
	}
	if p.Max != nil && n > *p.Max {
		n = *p.Max
	}
	return n
}

// coerceValue converts one argument to the type the schema declares for it. ok is
// false when the value is already right or cannot be converted, in which case it
// is left for the validator.
func coerceValue(raw json.RawMessage, spec propSpec) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false
	}
	var s string
	if json.Unmarshal(trimmed, &s) != nil {
		// Not a string. A native number or boolean aimed at a string property
		// (get_digest.limit takes "all" too) becomes its string form; a native
		// integer aimed at an integer property is clamped into range.
		switch spec.Type {
		case "string":
			if bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("false")) ||
				trimmed[0] == '-' || (trimmed[0] >= '0' && trimmed[0] <= '9') {
				return mustMarshalString(string(trimmed)), true
			}
		case "integer":
			if n, err := strconv.ParseInt(string(trimmed), 10, 64); err == nil {
				if c := spec.clampInt(n); c != n {
					return json.RawMessage(strconv.FormatInt(c, 10)), true
				}
			}
		}
		return nil, false
	}
	s = strings.TrimSpace(s)
	switch spec.Type {
	case "boolean":
		switch strings.ToLower(s) {
		case "true":
			return json.RawMessage("true"), true
		case "false":
			return json.RawMessage("false"), true
		}
	case "integer":
		if intLiteral.MatchString(s) {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				return json.RawMessage(strconv.FormatInt(spec.clampInt(n), 10)), true
			}
		}
	case "array":
		var items []string
		if strings.HasPrefix(s, "[") && json.Unmarshal([]byte(s), &items) == nil {
			return mustMarshal(trimmedNonEmpty(items)), true
		}
		return mustMarshal(trimmedNonEmpty(strings.Split(s, ","))), true
	}
	return nil, false
}

func trimmedNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func mustMarshal(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// coerceArguments rewrites args in place against one tool's property types.
func coerceArguments(args json.RawMessage, propTypes map[string]propSpec) json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(args, &m) != nil || m == nil {
		return args
	}
	changed := false
	for k, v := range m {
		if ty, ok := propTypes[k]; ok {
			if nv, did := coerceValue(v, ty); did {
				m[k] = nv
				changed = true
			}
		}
	}
	if !changed {
		return args
	}
	out, err := json.Marshal(m)
	if err != nil {
		return args
	}
	return out
}

// installLegacyCoercion runs coerceArguments on every tools/call before the SDK
// validates it. The tool property types are read once, from the registered schemas.
func (r *Registry) installLegacyCoercion() {
	var once sync.Once
	types := map[string]map[string]propSpec{}
	r.Server.MCP.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			cp, ok := req.GetParams().(*mcp.CallToolParamsRaw)
			if !ok || cp == nil || len(cp.Arguments) == 0 {
				return next(ctx, method, req)
			}
			once.Do(func() {
				ss, _ := req.GetSession().(*mcp.ServerSession)
				listed, err := next(ctx, "tools/list", &mcp.ListToolsRequest{Session: ss, Params: &mcp.ListToolsParams{}})
				if err != nil {
					return
				}
				if lt, ok := listed.(*mcp.ListToolsResult); ok {
					for _, tool := range lt.Tools {
						types[tool.Name] = schemaPropertySpecs(tool.InputSchema)
					}
				}
			})
			if pt := types[cp.Name]; pt != nil {
				cp.Arguments = coerceArguments(cp.Arguments, pt)
			}
			return next(ctx, method, req)
		}
	})
}

func schemaPropertySpecs(inputSchema any) map[string]propSpec {
	raw, _ := json.Marshal(inputSchema)
	var s struct {
		Properties map[string]struct {
			Type    string   `json:"type"`
			Minimum *float64 `json:"minimum"`
			Maximum *float64 `json:"maximum"`
		} `json:"properties"`
	}
	if json.Unmarshal(raw, &s) != nil {
		return nil
	}
	out := make(map[string]propSpec, len(s.Properties))
	for k, v := range s.Properties {
		spec := propSpec{Type: v.Type}
		if v.Minimum != nil {
			n := int64(*v.Minimum)
			spec.Min = &n
		}
		if v.Maximum != nil {
			n := int64(*v.Maximum)
			spec.Max = &n
		}
		out[k] = spec
	}
	return out
}
