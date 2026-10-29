# Go builder stage. Runs on the build machine and cross-compiles for the target platform.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS go-builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X github.com/klolarion/k_watch/backend/handler.Version=${VERSION}" \
    -o main .

# Node.js builder stage. The output is static files, so it never needs the target platform.
FROM --platform=$BUILDPLATFORM node:22-slim AS node-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Final stage. The Go binary is static, so this image only needs CA certificates.
FROM alpine:3.24
LABEL org.opencontainers.image.source="https://github.com/klolarion/k_watch" \
      org.opencontainers.image.description="Namespace map for one Kubernetes cluster" \
      org.opencontainers.image.licenses="Apache-2.0"
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=go-builder /app/backend/main ./
COPY --from=node-builder /app/frontend/dist ./static

ENV GIN_MODE=release
USER 65532:65532
EXPOSE 8282
CMD ["./main"]
