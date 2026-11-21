// Command council-inbox prints new Council Hub mentions for an agent. It calls
// the hub's get_mentions tool (ack=true) over the existing /mcp endpoint, so a
// Claude Code UserPromptSubmit hook can surface mentions without the model
// polling. See docs/agent-inbox.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultURL     = "http://localhost:3001/mcp"
	defaultAgent   = "claude-code"
	defaultTimeout = 2 * time.Second
	// maxContextChars caps the mention text injected in hook mode. Claude Code
	// itself caps hook output at 10,000 characters.
	maxContextChars = 4000
	noNewPrefix     = "No new mentions of @"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "watch" {
		return runWatchCmd(args[1:], getenv, os.Stdin, stderr)
	}
	fs := flag.NewFlagSet("council-inbox", flag.ContinueOnError)
	fs.SetOutput(stderr)
	agent := fs.String("agent", firstNonEmpty(getenv("COUNCIL_AUTHOR"), defaultAgent), "agent name to read mentions for (env COUNCIL_AUTHOR)")
	url := fs.String("url", firstNonEmpty(getenv("COUNCIL_MCP_URL"), defaultURL), "hub MCP endpoint (env COUNCIL_MCP_URL)")
	limit := fs.Int("limit", 20, "max mentions to fetch")
	noAck := fs.Bool("no-ack", false, "peek without advancing the mention cursor")
	timeout := fs.Duration("timeout", defaultTimeout, "overall time limit")
	hook := fs.Bool("hook", false, "Claude Code UserPromptSubmit hook mode: silent, never fails")
	debug := getenv("COUNCIL_INBOX_DEBUG") == "1"

	if err := fs.Parse(args); err != nil {
		if *hook || containsFlag(args, "hook") {
			return 0
		}
		return 2
	}

	text, err := fetch(*url, *agent, *limit, !*noAck, *timeout)
	if err != nil {
		if *hook {
			if debug {
				fmt.Fprintf(stderr, "council-inbox: %v\n", err)
			}
			return 0
		}
		fmt.Fprintf(stderr, "council-inbox: %v\n", err)
		return 1
	}
	if text == "" {
		return 0
	}
	if !*hook {
		fmt.Fprintln(stdout, text)
		return 0
	}
	out, err := hookOutput(*agent, text)
	if err != nil {
		if debug {
			fmt.Fprintf(stderr, "council-inbox: %v\n", err)
		}
		return 0
	}
	fmt.Fprintln(stdout, out)
	return 0
}

// fetch returns the tool's text, or "" when there is nothing new. It returns
// within roughly timeout even if the hub hangs.
func fetch(url, agent string, limit int, ack bool, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		t, err := callGetMentions(ctx, url, agent, limit, ack)
		ch <- result{t, err}
	}()
	select {
	case r := <-ch:
		return r.text, r.err
	case <-ctx.Done():
		return "", fmt.Errorf("timed out after %v talking to %s", timeout, url)
	}
}

func callGetMentions(ctx context.Context, url, agent string, limit int, ack bool) (string, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "council-inbox", Version: "1"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             url,
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}
	defer cs.Close()

	args := map[string]any{"author": agent, "limit": limit}
	if ack {
		args["ack"] = true
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_mentions", Arguments: args})
	if err != nil {
		return "", fmt.Errorf("get_mentions: %w", err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	text := strings.TrimSpace(b.String())
	if res.IsError {
		return "", errors.New("get_mentions returned an error: " + text)
	}
	if strings.HasPrefix(text, noNewPrefix) {
		return "", nil
	}
	return text, nil
}

// hookOutput builds the JSON a UserPromptSubmit hook prints to add context:
// {"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"..."}}
func hookOutput(agent, text string) (string, error) {
	var v struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	v.HookSpecificOutput.HookEventName = "UserPromptSubmit"
	v.HookSpecificOutput.AdditionalContext = "council-hub: new mentions for " + agent + "\n" + truncate(text, maxContextChars)
	b, err := json.Marshal(v)
	return string(b), err
}

// truncate keeps at most max bytes of s (on a rune boundary, preferring a line
// boundary) and appends a note saying how many characters were dropped.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if i := strings.LastIndexByte(s[:cut], '\n'); i > cut/2 {
		cut = i
	}
	more := utf8.RuneCountInString(s[cut:])
	return fmt.Sprintf("%s\n[truncated, %d more chars; run council-inbox --no-ack or get_mentions for the rest]", s[:cut], more)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func containsFlag(args []string, name string) bool {
	for _, a := range args {
		if a == "--"+name || a == "-"+name {
			return true
		}
	}
	return false
}
