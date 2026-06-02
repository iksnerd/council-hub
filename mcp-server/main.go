package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"council-hub/internal/council"
	"council-hub/internal/handlers"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type clusterNodeInfo struct {
	Node    string `json:"node"`
	Version string `json:"version"`
}

type clusterNodesResult struct {
	Nodes           []clusterNodeInfo
	VersionMismatch bool
}

// clusterNodes queries Phoenix for the list of connected Erlang nodes with
// their versions. Returns nil if Phoenix is unavailable.
// portFromAddr extracts the port from a bind address like ":3001" or
// "0.0.0.0:3001", defaulting to "3001" when none can be determined.
func portFromAddr(addr string) string {
	if i := strings.LastIndex(addr, ":"); i >= 0 && i < len(addr)-1 {
		return addr[i+1:]
	}
	return "3001"
}

func clusterNodes(phoenixURL string, client *http.Client) *clusterNodesResult {
	if phoenixURL == "" || client == nil {
		return nil
	}
	resp, err := client.Get(phoenixURL + "/api/internal/cluster/nodes")
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	var payload struct {
		Nodes           []clusterNodeInfo `json:"nodes"`
		VersionMismatch bool              `json:"version_mismatch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	return &clusterNodesResult{
		Nodes:           payload.Nodes,
		VersionMismatch: payload.VersionMismatch,
	}
}

// healthHandler exposes a JSON snapshot of database integrity state. Returns
// 200 even when self-heals have occurred — heals are recoverable; the absence
// of a recent integrity check is what monitoring should alert on.
func healthHandler(cs *council.Server, phoenixURL string, httpClient *http.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs.Mu.RLock()
		last := cs.LastIntegrityCheck
		cs.Mu.RUnlock()
		body := map[string]any{
			"status":                "ok",
			"version":               council.Version,
			"last_integrity_check":  last.UTC().Format(time.RFC3339),
			"heal_count_since_boot": atomic.LoadUint64(&cs.HealCount),
			"now":                   time.Now().UTC().Format(time.RFC3339),
		}
		if result := clusterNodes(phoenixURL, httpClient); result != nil {
			body["cluster_nodes"] = result.Nodes
			if result.VersionMismatch {
				body["cluster_warning"] = "version mismatch detected across cluster nodes — upgrade all nodes to the same version"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}
}

// loggingMiddleware logs every incoming MCP request with method name, duration,
// and (for tools/call) the tool name. Errors are logged at WARN; everything else
// at DEBUG so that COUNCIL_DEBUG=1 surfaces request traffic without spamming
// production logs.
func loggingMiddleware(logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			start := time.Now()
			toolName := ""
			if method == "tools/call" {
				if p, ok := req.GetParams().(*mcp.CallToolParams); ok && p != nil {
					toolName = p.Name
				}
			}
			res, err := next(ctx, method, req)
			dur := time.Since(start)
			if err != nil {
				logger.Warn("mcp request failed", "method", method, "tool", toolName, "duration_ms", dur.Milliseconds(), "error", err)
			} else if toolName != "" {
				logger.Debug("mcp tool call", "tool", toolName, "duration_ms", dur.Milliseconds())
			} else {
				logger.Debug("mcp request", "method", method, "duration_ms", dur.Milliseconds())
			}
			return res, err
		}
	}
}

func main() {
	logLevel := slog.LevelInfo
	if os.Getenv("COUNCIL_DEBUG") == "1" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	dbPath := os.Getenv("COUNCIL_DB")
	if dbPath == "" {
		dbPath = "council.db"
	}

	cs, err := council.NewServer(dbPath, logger)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer func() { _ = cs.DB.Close() }()

	// Initialize embedder via Ollama
	if ollamaURL := os.Getenv("COUNCIL_OLLAMA_URL"); ollamaURL != "" {
		embedder := council.NewOllamaEmbedder(ollamaURL, os.Getenv("COUNCIL_EMBED_MODEL"), logger)
		cs.Embedder = embedder
		logger.Info("Semantic search enabled", "provider", "ollama", "url", ollamaURL, "model", embedder.Model)
	} else {
		logger.Info("Semantic search disabled (set COUNCIL_OLLAMA_URL to enable)")
	}

	phoenixURL := os.Getenv("COUNCIL_PHOENIX_URL")
	if phoenixURL == "" {
		phoenixURL = "http://127.0.0.1:4000"
	}

	// Peer MCP port for cross-node write proxying. Defaults to the port this node
	// serves MCP on (COUNCIL_HTTP_ADDR), or 3001; override with COUNCIL_PEER_MCP_PORT.
	peerMCPPort := os.Getenv("COUNCIL_PEER_MCP_PORT")
	if peerMCPPort == "" {
		peerMCPPort = portFromAddr(os.Getenv("COUNCIL_HTTP_ADDR"))
	}

	reg := &handlers.Registry{
		Server:        cs,
		HTTPClient:    &http.Client{Timeout: 10 * time.Second},
		PhoenixURL:    phoenixURL,
		PeerMCPPort:   peerMCPPort,
		ClusterSecret: os.Getenv("RELEASE_COOKIE"),
	}
	reg.RegisterTools()
	reg.RegisterResources()

	cs.MCP.AddReceivingMiddleware(loggingMiddleware(logger))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start the Knowledge Linter
	go cs.RunJanitor(ctx)

	// Backfill embeddings on startup + every 10 min (retries if Ollama was down)
	go cs.RunEmbedBackfill(ctx)

	transport := os.Getenv("COUNCIL_TRANSPORT")
	if transport == "" {
		transport = "stdio"
	}

	switch transport {
	case "http", "sse":
		addr := os.Getenv("COUNCIL_HTTP_ADDR")
		if addr == "" {
			addr = ":3001"
		}

		mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			return cs.MCP
		}, &mcp.StreamableHTTPOptions{
			Logger: logger,
		})

		mux := http.NewServeMux()
		mux.Handle("/mcp", mcpHandler)
		mux.Handle("/mcp/", mcpHandler)
		mux.HandleFunc("/health", healthHandler(cs, phoenixURL, reg.HTTPClient))
		// Cross-node write receiver (authenticated by the shared cluster secret).
		mux.HandleFunc("/api/internal/post_to_room", reg.InternalPostHandler())
		// UI write endpoint (localhost-only, no auth — for the Phoenix dashboard compose box).
		mux.HandleFunc("/api/ui/post", reg.UIPostHandler())

		httpServer := &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		go func() {
			<-ctx.Done()
			logger.Info("Shutting down HTTP server")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				logger.Error("HTTP server shutdown error", "error", err)
			}
		}()

		logger.Info("Council Hub starting (HTTP)", "db", dbPath, "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}

	default:
		logger.Info("Council Hub starting (stdio)", "db", dbPath)
		if err := cs.MCP.Run(ctx, &mcp.StdioTransport{}); err != nil {
			if err.Error() != "EOF" {
				log.Fatalf("Server error: %v", err)
			}
		}
	}

	fmt.Println()
	logger.Info("Council Hub shutdown")
}
