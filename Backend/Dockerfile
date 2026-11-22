# 本番の既定ターゲットは最後の runtime。Go SDK も air もコンパイラも含めない。
# ローカルの compose は target: dev（Go と air がある）。デプロイの docker build は
# ターゲット未指定なので runtime だけがイメージになる。

FROM golang:1.26-bookworm AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto

# このモジュールは cgo を使わない。CGO_ENABLED=0 ならコンパイラは要らない。
RUN apt-get update && apt-get install -y --no-install-recommends \
      git \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/server ./cmd/server

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/migrate ./cmd/migrate

# プロンプト版の設定だけを確かめるコマンド。本番デプロイでサービスを差し替える前に実行する（SOCAIAGENT-661）
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/promptcheck ./cmd/promptcheck

# OCR の Python 依存はここで作り、実行イメージへ中身だけコピーする。
# PyMuPDF 1.20.2 は cp311 の wheel が無く、ソースビルドにコンパイラが要る（#1142）。
# コンパイラはこのステージに留め、本番イメージには置かない。
FROM debian:bookworm-slim AS ocr

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      python3 \
      python3-venv \
      python3-dev \
      swig \
      build-essential \
      libfreetype6-dev \
      libjpeg62-turbo-dev \
      libopenjp2-7-dev \
      zlib1g-dev \
 && rm -rf /var/lib/apt/lists/*

RUN python3 -m venv /opt/ocr

# requirements.txt が変わらない限り、この層を再利用する。
COPY scripts/ /tmp/ocr-scripts/
RUN if [ -d /tmp/ocr-scripts/wheels ] && [ "$(ls -A /tmp/ocr-scripts/wheels)" ]; then \
      /opt/ocr/bin/pip install --no-index --find-links=/tmp/ocr-scripts/wheels -r /tmp/ocr-scripts/requirements.txt ; \
    else \
      /opt/ocr/bin/pip install --prefer-binary --default-timeout=600 --retries=8 -r /tmp/ocr-scripts/requirements.txt ; \
    fi \
 && rm -rf /tmp/ocr-scripts

# ローカル compose 用。ソースはマウントされ、entrypoint が go run と air を使う。
FROM golang:1.26-bookworm AS dev

WORKDIR /app

ENV GOTOOLCHAIN=auto
ENV ANNOTATION_FONT_PATH=/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      tzdata \
      python3 \
      libreoffice \
      fonts-noto-cjk \
      libgl1 \
      libglib2.0-0 \
      libgomp1 \
      libfreetype6 \
      libjpeg62-turbo \
      libopenjp2-7 \
 && rm -rf /var/lib/apt/lists/*

ARG AIR_VERSION=v1.62.0
RUN GOBIN=/usr/local/bin go install github.com/air-verse/air@${AIR_VERSION}

COPY --from=ocr /opt/ocr /opt/ocr
ENV PATH="/opt/ocr/bin:${PATH}"

RUN groupadd --gid 10001 app \
 && useradd --uid 10001 --gid 10001 --create-home --home-dir /home/app --shell /bin/sh app

ENV GOPATH=/home/app/go
ENV GOCACHE=/home/app/.cache/go-build

COPY --from=builder --chown=app:app /app /app
COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/migrate /bin/migrate
RUN chmod +x /bin/server /bin/migrate \
 && mkdir -p /app/storage /home/app/go/pkg/mod /home/app/.cache/go-build \
 && chown -R app:app /app/storage /home/app

USER app
EXPOSE 8080
CMD ["/bin/server"]

# 本番。デプロイはターゲットを指定しないので、このステージがイメージになる。
FROM debian:bookworm-slim AS runtime

WORKDIR /app

ENV ANNOTATION_FONT_PATH=/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc
ENV PATH="/opt/ocr/bin:${PATH}"

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      tzdata \
      python3 \
      libreoffice \
      fonts-noto-cjk \
      libgl1 \
      libglib2.0-0 \
      libgomp1 \
      libfreetype6 \
      libjpeg62-turbo \
      libopenjp2-7 \
 && rm -rf /var/lib/apt/lists/*

COPY --from=ocr /opt/ocr /opt/ocr

# venv を別ステージからコピーしても、職務経歴書の注釈が使うライブラリが読めることをここで落とす。
# wheel 同梱の .so は ldd だと隣のファイルを見落として欠落に見えるので、import で確認する。
RUN python3 -c "import fitz, PIL, numpy"

RUN groupadd --gid 10001 app \
 && useradd --uid 10001 --gid 10001 --create-home --home-dir /home/app --shell /bin/sh app

COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/migrate /bin/migrate
COPY --from=builder /bin/promptcheck /bin/promptcheck
COPY --chown=app:app config/match_reason_templates.json /app/config/match_reason_templates.json
COPY --chown=app:app config/l1_catalog_seed.sample.csv /app/config/l1_catalog_seed.sample.csv
COPY --chown=app:app scripts/ocr_extract.py scripts/annotate_pdf.py /app/scripts/

RUN chmod +x /bin/server /bin/migrate /bin/promptcheck \
 && mkdir -p /app/storage \
 && chown -R app:app /app/storage /home/app

USER app
EXPOSE 8080
CMD ["/bin/server"]
