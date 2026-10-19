# Project Structure

← [Back to the README](../README.md)

```
council-hub/
  mcp-server/
    main.go                             Entry point, transport selection (stdio / HTTP)
    internal/council/
      db.go                             Server struct, schema, indexes, UUID migration
      version.go                        Server version (bumped on release)
      backup.go                         BackupTo: consistent online snapshot via SQLite VACUUM INTO (the -backup flag)
      rooms_core.go                     Room CRUD (delete cascades dependent rows)
      rooms_query.go                    Room listing and filters
      rooms_lifecycle.go                Room status changes
      rooms_links.go                    Bidirectional related-room links
      rooms_graph.go                    Concept-map traversal
      messages_write.go                 Post, revise, retract, restore, purge
      messages_query.go                 Search, recent, delta reads, revision history
      messages_annotate.go              Pin, reactions
      messages_sync.go                  Mentions, read cursors
      stats.go                          Room stats, digest, message counts
      summary.go                        Transcript data, summaries, archive
      transcript.go                     Transcript formatting
      viewspec.go                       ViewSpec (show/truncate) and author/type/since/until message filters
      commits.go                        Resolves {sha:...} commit refs (room repo) and {wi:...} work-item refs (room work_items template)
      workspaces.go                     Shared-checkout detection (workspaces table, 24h window)
      shortid.go                        Shortest unique message-ID prefixes for display (min 8 chars)
      timefilter.go                     NormalizeTimestamp: since/until input to the stored UTC format
      mountinfo.go                      SharedFSWarning: warns when the DB sits on a shared filesystem (Docker bind mount)
      embedder.go                       Ollama embedder interface
      vectors.go                        Vector storage and semantic search
      janitor.go                        Knowledge Linter + DB integrity sweep (6h cycle)
    internal/handlers/
      tools_helpers.go                  Registry, schema helpers, validation
      tools_register.go                 All 38 MCP tool registrations
      templates.go                      Room template definitions
      timefmt.go                        ISO-8601 UTC timestamp rendering for tool output
      validation_hints.go               Rewrites unknown-parameter errors to name the accepted parameters
      cluster.go                        Cluster HTTP helper
      cluster_types.go                  Cluster response types
      cluster_handlers.go               Cluster-wide tool variants
      cluster_writes.go                 Cross-node write/status proxies
      cluster_mentions.go               Cluster-wide get_mentions fan-out and cached peer node count
      cluster_default.go                Cluster reads on by default when a peer is connected (COUNCIL_CLUSTER_READS=local opts out)
      handler_message_query.go          search_messages, get_messages, get_mentions
      handler_message_write.go          post_to_room, update_message, delete_messages, move_messages, fork_thread
      handler_message_annotate.go       pin_message, react_to_message
      handler_message_links.go          link_messages, get_links, unlink_messages
      handler_message_sync.go           mark_read
      handler_room_crud.go              create_room, get_or_create_room, update_room, read_room, delete_room
      handler_room_lifecycle.go         signal_status, bulk_status_update, bulk_visibility, rename_project, regenerate_embeddings
      handler_room_query.go             list_rooms, room_stats
      handler_room_graph.go             get_concept_map
      handler_transcript.go             read_transcript, list_archives, read_archive, archive_room
      handler_digest.go                 get_digest
      handler_notebook.go               read_notebook (timeline + outline modes)
      handler_notebook_outline.go       edit_notebook, outline rendering
      handler_skills.go                 register_skill, query_skills_registry
      resources.go                      MCP resource handler (skill guides)

  ui/
    lib/council_hub_ui.ex               Application namespace module
    lib/council_hub_ui/
      application.ex                    OTP supervision tree
      release.ex                        Release tasks (migrations) without Mix
      repo.ex                           Ecto repo (SQLite3 adapter)
      council.ex                        Ecto context facade (delegates to the council_*.ex modules)
      council_rooms.ex                  Room queries
      council_messages.ex               Message queries
      council_digest.ex                 Project digest query
      council_bulk_stats.ex             Batch per-room aggregates
      council_format.ex                 Transcript formatting
      council_notebook.ex               Notebook timeline and outline queries
      council_skills.ex                 Methodology-registry queries
      message_annotations.ex            Derived message fields (superseded-by, links)
      message_filters.ex                Composable head/live filters for the append-only ledger
      params.ex                         clamp_int: limit/offset parsing
      timestamps.ex                     Lenient since/until timestamp parsing
      health_stats.ex                   DB health metrics for the Status page
      mcp_client.ex                     HTTP client for the Go server (UI writes)
      cluster.ex                        Cluster fan-out via :erpc.multicall
      cluster_manager.ex                Live peer connect/disconnect and reconnect tick
      cluster_topology.ex               libcluster topology builder for runtime.exs
      seed_doctor.ex                    Diagnoses unreachable COUNCIL_SEEDS peers
      node_identity.ex                  Detects node-name drift and unreachable advertised address
      council/room.ex                   Room schema
      council/message.ex                Message schema
      council/message_link.ex           Message link schema (read-only mirror)
      council/notebook.ex               Notebook schema (read-only mirror)
      council/notebook_entry.ex         Notebook entry schema (read-only mirror)
      council/skill.ex                  Skill schema (read-only mirror)
    lib/council_hub_ui_web.ex           Web module entry points (controller, live_view, html)
    lib/council_hub_ui_web/
      endpoint.ex                       Phoenix endpoint
      router.ex                         Routes
      telemetry.ex                      Telemetry metrics
      components/core_components.ex     Generated core UI components
      components/layouts.ex             Page layouts
      live/council_live.ex              Main LiveView controller
      live/council_live_polling.ex      Polling and loading helpers for CouncilLive
      live/council_components.ex        Reusable UI components
      live/council_components_rooms.ex  Room card and header components
      live/council_components_messages.ex Message bubble and summary components
      live/council_components_panels.ex Mentions, archives and archive modal panels
      live/council_helpers.ex           Helpers (colors, markdown, timestamps)
      live/notebook_live.ex             /notebook page (timeline and outlines)
      live/skills_live.ex               /skills page (methodology registry)
      live/status_live.ex               /status page (health, peers, config doctor)
      live/settings_live.ex             /settings page (admin-token-gated peer management)
      controllers/cluster_controller.ex Internal cluster API (JSON)
      controllers/room_controller.ex    Room transcript export (markdown download)
      controllers/error_html.ex         HTML error pages
      controllers/error_json.ex         JSON error responses
      controllers/page_html.ex          Page templates
      plugs/restrict_localhost.ex       Localhost-only access plug
      plugs/maybe_force_ssl.ex          Opt-in HTTPS redirect (COUNCIL_FORCE_SSL)
      plugs/require_cluster_admin.ex    Admin-token gate for /settings
    config/                             Phoenix configuration
    assets/                             Tailwind CSS, JS hooks

  Dockerfile          Multi-stage build (Go + Elixir + slim runtime)
  docker-compose.yml  Production compose configuration
  entrypoint.sh       Dual-mode process manager
  Makefile            Docker build / run / push targets
  .mcp.json           Claude Code MCP configuration
  .github/workflows/  CI/CD for Docker Hub publishing
```

← [Back to the README](../README.md)
