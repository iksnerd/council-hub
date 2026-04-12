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

RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev libsqlite3-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /go-app

COPY mcp-server/go.mod mcp-server/go.sum ./
RUN go mod download
COPY mcp-server/*.go ./
COPY mcp-server/internal/ ./internal/
RUN CGO_ENABLED=1 go build -tags sqlite_fts5 -o council-hub .

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

ARG TARGETARCH
ARG ORT_VERSION=1.22.0

RUN apt-get update && apt-get install -y --no-install-recommends \
    libstdc++6 openssl ca-certificates wget bash \
    && rm -rf /var/lib/apt/lists/*

# Install ONNX Runtime from Microsoft GitHub releases
RUN ORT_ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "aarch64" || echo "x64") && \
    wget -q -O /tmp/ort.tgz \
      "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}.tgz" && \
    tar -xzf /tmp/ort.tgz -C /tmp && \
    cp /tmp/onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}/lib/libonnxruntime.so.${ORT_VERSION} /usr/lib/ && \
    ln -s /usr/lib/libonnxruntime.so.${ORT_VERSION} /usr/lib/libonnxruntime.so && \
    rm -rf /tmp/ort.tgz /tmp/onnxruntime-*

WORKDIR /app

RUN groupadd -g 1000 council && \
    useradd -u 1000 -g council -m council && \
    mkdir -p /data && chown council:council /data

# Download ONNX MiniLM model for built-in semantic search
RUN mkdir -p /app/models/all-MiniLM-L6-v2 && \
    wget -q -O /app/models/all-MiniLM-L6-v2/model.onnx \
      "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx" && \
    wget -q -O /app/models/all-MiniLM-L6-v2/tokenizer.json \
      "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"

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
ENV COUNCIL_ONNX_MODEL_DIR=/app/models/all-MiniLM-L6-v2
ENV ONNXRUNTIME_LIB_PATH=/usr/lib/libonnxruntime.so
ENV COUNCIL_DB=/data/council.db
ENV COUNCIL_TRANSPORT=http
ENV COUNCIL_HTTP_ADDR=:3001

# Phoenix config
ENV MIX_ENV=prod
ENV PHX_SERVER=true
ENV PHX_HOST=localhost
ENV PORT=4000
ENV COUNCIL_DB_PATH=/data/council.db

# Distributed Erlang config
ENV RELEASE_COOKIE=council
ENV RELEASE_DISTRIBUTION=name
ENV RELEASE_NODE=council_hub@127.0.0.1
ENV ELIXIR_ERL_OPTIONS="-kernel inet_dist_listen_min 9000 -kernel inet_dist_listen_max 9000"

VOLUME /data
EXPOSE 4000 3001 4369 9000

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD [ "$COUNCIL_TRANSPORT" = "stdio" ] || wget --no-verbose --tries=1 --spider http://localhost:4000 || exit 1

ENTRYPOINT ["entrypoint.sh"]