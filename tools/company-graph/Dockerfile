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

EXPOSE 9100

CMD ["/bin/company-graph"]
