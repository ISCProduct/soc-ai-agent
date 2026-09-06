# Build stage
FROM golang:1.24-bookworm AS builder

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

# Runtime stage
FROM golang:1.24-bookworm

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
    swig \
    build-essential \
    python3-dev \
 && rm -rf /var/lib/apt/lists/*

# Install Air for local Docker Compose hot reload.
ARG AIR_VERSION=v1.62.0
RUN go install github.com/air-verse/air@${AIR_VERSION}


# Optional: copy full source for debugging/hot reload
COPY --from=builder /app /app

# Copy built binaries
COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/migrate /bin/migrate

# Make them executable
RUN chmod +x /bin/server /bin/migrate

# Install OCR dependencies (use local wheels if provided)
# bookworm の pip は PEP 668 で externally-managed のため --break-system-packages が必要。
# PyMuPDF 1.20.2 は cp311 の wheel を提供しておらず、上で入れた swig / build-essential /
# python3-dev を使ってソースからビルドされる（paddleocr 2.7.0.3 が PyMuPDF<1.21.0 を要求するため
# wheel のある新しい PyMuPDF へは上げられない）。#1142 参照。
RUN if [ -d /app/scripts/wheels ] && [ "$(ls -A /app/scripts/wheels)" ]; then \
      python3 -m pip install --break-system-packages --no-index --find-links=/app/scripts/wheels -r /app/scripts/requirements.txt ; \
    else \
      python3 -m pip install --break-system-packages --prefer-binary --default-timeout=600 --retries=8 -r /app/scripts/requirements.txt ; \
    fi

EXPOSE 8080

CMD ["/bin/server"]
