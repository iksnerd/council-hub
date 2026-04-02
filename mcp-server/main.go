package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"council-hub/internal/council"
	"council-hub/internal/handlers"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	defer cs.DB.Close()

	phoenixURL := os.Getenv("COUNCIL_PHOENIX_URL")
	if phoenixURL == "" {
		phoenixURL = "http://127.0.0.1:4000"
	}

	reg := &handlers.Registry{
		Server:     cs,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		PhoenixURL: phoenixURL,
	}
	reg.RegisterTools()
	reg.RegisterResources()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// TODO: Re-enable once we have a summarization strategy that preserves
	// decisions, actions, and code blocks instead of losing context.
	// go cs.RunJanitor(ctx)

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

		handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			return cs.MCP
		}, &mcp.StreamableHTTPOptions{
			Logger: logger,
		})

		httpServer := &http.Server{
			Addr:         addr,
			Handler:      handler,
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
