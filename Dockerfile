# Build stage
FROM golang:1.26.5-alpine3.24 AS builder

RUN apk add --no-cache git nodejs npm make

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build frontend
RUN cd frontend && npm ci && npm run build
RUN mkdir -p web/frontend && cp -r frontend/dist web/frontend/

# Build server edition binary
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build \
    -tags server \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE} -X github.com/smart-mcp-proxy/mcpproxy-go/internal/httpapi.buildVersion=${VERSION} -X github.com/smart-mcp-proxy/mcpproxy-go/internal/updatecheck.buildChannel=docker -s -w" \
    -o /mcpproxy ./cmd/mcpproxy

# Runtime stage. Alpine provides /bin/sh for the Runabot web terminal.
FROM alpine:3.24.1

RUN apk add --no-cache ca-certificates && \
    addgroup -g 65532 -S nonroot && \
    adduser -u 65532 -S -G nonroot nonroot

COPY --from=builder /mcpproxy /usr/local/bin/mcpproxy

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["mcpproxy", "serve", "--listen", "0.0.0.0:8080"]
