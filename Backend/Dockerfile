# Build stage
FROM golang:1.25-bookworm AS builder

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
FROM golang:1.25-bookworm

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

# Air はローカルの Docker Compose ホットリロード用。
# GOPATH ではなく /usr/local/bin へ置き、どのユーザーからも PATH で引けるようにする。
ARG AIR_VERSION=v1.62.0
RUN GOBIN=/usr/local/bin go install github.com/air-verse/air@${AIR_VERSION}


# OCR 依存はソース全体のコピーより前に入れる。
# ここを後ろに置くと、Go を1行直しただけでも直前の COPY 層が無効化され、
# PaddleOCR の再インストールと PyMuPDF のソースビルドが毎デプロイで再実行される。
# scripts/ だけを先にコピーすることで、requirements.txt が変わらない限り層が再利用される。
#
# bookworm の pip は PEP 668 で externally-managed のため --break-system-packages が必要。
# PyMuPDF 1.20.2 は cp311 の wheel を提供しておらず、上で入れた swig / build-essential /
# python3-dev を使ってソースからビルドされる（paddleocr 2.7.0.3 が PyMuPDF<1.21.0 を要求するため
# wheel のある新しい PyMuPDF へは上げられない）。#1142 参照。
COPY scripts/ /app/scripts/
RUN if [ -d /app/scripts/wheels ] && [ "$(ls -A /app/scripts/wheels)" ]; then \
      python3 -m pip install --break-system-packages --no-index --find-links=/app/scripts/wheels -r /app/scripts/requirements.txt ; \
    else \
      python3 -m pip install --break-system-packages --prefer-binary --default-timeout=600 --retries=8 -r /app/scripts/requirements.txt ; \
    fi

# 非 root 実行用のユーザー(#1410)。
# 任意ファイル書き込み系の不具合が出たときの被害範囲を抑える。
# 8080 番で待ち受けるため特権ポートの問題は無い。
RUN groupadd --gid 10001 app \
 && useradd --uid 10001 --gid 10001 --create-home --home-dir /home/app --shell /bin/sh app

# Go のキャッシュ類を /root から app のホーム配下へ移す。
# /root は 0700 で、非 root からは GOPATH も GOCACHE も書けない。
ENV GOPATH=/home/app/go
ENV GOCACHE=/home/app/.cache/go-build

# 実行時に config/match_reason_templates.json 等を相対パスで読むため、ソース一式が必要。
COPY --from=builder --chown=app:app /app /app

# Copy built binaries
COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/migrate /bin/migrate

# Make them executable
RUN chmod +x /bin/server /bin/migrate

# 書き込み先の所有権を合わせる。
# storage は履歴書の保存先(resume_service.go の既定 storage/resumes が /app 相対)。
RUN mkdir -p /app/storage /home/app/go/pkg/mod /home/app/.cache/go-build \
 && chown -R app:app /app/storage /home/app

USER app

EXPOSE 8080

CMD ["/bin/server"]
