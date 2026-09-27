# Project Structure

← [Back to the README](../README.md)

```
council-hub/
  mcp-server/
    main.go                             Entry point, transport selection (stdio / HTTP)
    internal/council/
      db.go                             Server struct, schema, indexes, UUID migration
      version.go                        Server version (bumped on release)
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
      embedder.go                       Ollama embedder interface
      vectors.go                        Vector storage and semantic search
      janitor.go                        Knowledge Linter + DB integrity sweep (6h cycle)
    internal/handlers/
      tools_helpers.go                  Registry, schema helpers, validation
      tools_register.go                 All 38 MCP tool registrations
      templates.go                      Room template definitions
      cluster.go                        Cluster HTTP helper
      cluster_types.go                  Cluster response types
      cluster_handlers.go               Cluster-wide tool variants
      cluster_writes.go                 Cross-node write/status proxies
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
    lib/council_hub_ui/
      council.ex                        Ecto context (queries, transcript formatting)
      cluster.ex                        Cluster fan-out via :erpc.multicall
      council/room.ex                   Room schema
      council/message.ex                Message schema
    lib/council_hub_ui_web/
      live/council_live.ex              Main LiveView controller
      live/council_components.ex        Reusable UI components
      live/council_helpers.ex           Helpers (colors, markdown, timestamps)
      controllers/cluster_controller.ex Internal cluster API (JSON)
      plugs/restrict_localhost.ex       Localhost-only access plug
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
