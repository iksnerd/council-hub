# ============================================================
# Council Hub — unified image (MCP server + Web UI)
#
#   HTTP mode (persistent service):
#     docker run -d -p 4000:4000 -p 3001:3001 \
#       -v ~/Documents/council-hub:/data council-hub
#
#   Stdio mode (CLI agents):
#     docker run -i --rm -e COUNCIL_TRANSPORT=stdio \
#       -v ~/Documents/council-hub:/data council-hub
#
#   MCP server: http://localhost:3001/mcp
#   Web UI:     http://localhost:4000
# ============================================================

# --- Stage 1: Build Go MCP server ---
FROM golang:1.25 AS go-builder

RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /go-app

COPY mcp-server/go.mod mcp-server/go.sum ./
RUN go mod download
COPY mcp-server/*.go ./
RUN CGO_ENABLED=1 go build -o council-hub .

# --- Stage 2: Build Elixir/Phoenix UI ---
FROM elixir:1.19-otp-28 AS elixir-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN mix local.hex --force && mix local.rebar --force

ENV MIX_ENV=prod

COPY ui/mix.exs ui/mix.lock ./
RUN mix deps.get --only $MIX_ENV
RUN mkdir config

COPY ui/config/config.exs ui/config/${MIX_ENV}.exs ui/config/runtime.exs config/
COPY ui/lib lib
COPY ui/priv priv
COPY ui/assets assets
COPY ui/rel rel

RUN mix deps.compile
RUN mix compile
RUN mix assets.deploy
RUN mix release

# --- Stage 3: Runtime ---
FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libstdc++6 openssl ca-certificates wget bash \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN groupadd -g 1000 council && \
    useradd -u 1000 -g council -m council && \
    mkdir -p /data && chown council:council /data

# Copy Go binary
COPY --from=go-builder /go-app/council-hub /usr/local/bin/council-hub

# Copy Elixir release
COPY --from=elixir-builder --chown=council:council /app/_build/prod/rel/council_hub_ui /app/ui/

# Copy entrypoint
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER council

# Shared config
ENV HOME=/app
ENV COUNCIL_DB=/data/council.db
ENV COUNCIL_TRANSPORT=http
ENV COUNCIL_HTTP_ADDR=:3001

# Phoenix config
ENV MIX_ENV=prod
ENV PHX_SERVER=true
ENV PHX_HOST=localhost
ENV PORT=4000
ENV COUNCIL_DB_PATH=/data/council.db

VOLUME /data
EXPOSE 4000 3001

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD [ "$COUNCIL_TRANSPORT" = "stdio" ] || wget --no-verbose --tries=1 --spider http://localhost:4000 || exit 1

ENTRYPOINT ["entrypoint.sh"]