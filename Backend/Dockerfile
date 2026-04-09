# Build stage
FROM golang:1.24-bullseye AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto
ENV ANNOTATION_FONT_PATH=/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
      git \
      build-essential \
 && rm -rf /var/lib/apt/lists/*

# Copy go mod files
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy source code
COPY . .

# Build main application
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -ldflags="-s -w" -o /bin/server ./cmd/server

# Build migrate script
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -ldflags="-s -w" -o /bin/migrate ./cmd/migrate

# Build seed script (optional)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -ldflags="-s -w" -o /bin/seed-large ./cmd/seed-large

# Runtime stage
FROM golang:1.24-bullseye

WORKDIR /app

ENV GOTOOLCHAIN=auto
# フォントパスを明示的に設定（ランタイムステージで有効）
# fonts-noto-cjk をインストール後のデフォルトパス
ENV ANNOTATION_FONT_PATH=/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc

# Install runtime dependencies (OCR + PDF conversion)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    python3 \
    python3-pip \
    libreoffice \
    fonts-noto-cjk \
    libgl1 \
    libglib2.0-0 \
    libgomp1 \
    chromium \
    chromium-driver \
 && rm -rf /var/lib/apt/lists/*

#
# NOTE: Hot-reload (air) is omitted to avoid Go toolchain version constraints.
#

# Optional: copy full source for debugging/hot reload
COPY --from=builder /app /app

# Copy built binaries
COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/migrate /bin/migrate
COPY --from=builder /bin/seed-large /bin/seed-large

# Make them executable
RUN chmod +x /bin/server /bin/migrate /bin/seed-large

# Install OCR dependencies (use local wheels if provided)
RUN if [ -d /app/scripts/wheels ] && [ "$(ls -A /app/scripts/wheels)" ]; then \
      python3 -m pip install --no-index --find-links=/app/scripts/wheels -r /app/scripts/requirements.txt ; \
    else \
      python3 -m pip install --prefer-binary --default-timeout=300 --retries=5 -r /app/scripts/requirements.txt ; \
    fi

EXPOSE 8080

CMD ["/bin/server"]
