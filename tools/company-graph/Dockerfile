# Build stage
FROM golang:1.24-bullseye AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto

RUN apt-get update && apt-get install -y --no-install-recommends \
      git \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/company-graph .

# Runtime stage
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      tzdata \
      chromium \
      chromium-driver \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/company-graph /bin/company-graph

# 非root実行用のユーザー(#1477)。9100 番で待ち受けるため特権ポートの問題は無い。
# chromedp は user-data-dir を os.TempDir() 配下に作るので /tmp のままで動く。
RUN groupadd --gid 10001 app \
 && useradd --uid 10001 --gid 10001 --create-home --home-dir /home/app --shell /bin/sh app

USER app

EXPOSE 9100

CMD ["/bin/company-graph"]
