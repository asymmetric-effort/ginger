# Builder stage
FROM ubuntu:24.04 AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git && \
    rm -rf /var/lib/apt/lists/*

# Install Go
ARG GO_VERSION=1.26.4
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | \
    tar -xz -C /usr/local
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /build

# Copy module files first for layer caching
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /ginger ./cmd/ginger

# Runtime stage
FROM gcr.io/distroless/base AS runtime

COPY --from=builder /ginger /ginger

USER nonroot:nonroot

ENTRYPOINT ["/ginger"]
