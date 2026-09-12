# syntax=docker/dockerfile:1

# ---- Go builder ----
FROM golang:1.26-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- Node builder ----
FROM node:24-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- Runtime ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=go-builder /out/server ./server
COPY --from=web-builder /src/web/dist ./web/dist

# Everything under /data is the single mounted volume -- the database,
# uploaded files (theme backgrounds, etc.), and now the log file all
# live under it by default, so a `docker-compose pull && up -d` (or any
# redeploy that recreates the container) never loses any of them. Only
# PORT and the SMTP_* vars are meaningfully overridden per-deployment in
# practice; see docker-compose.yml for the full list with explanations.
ENV PORT=8080
ENV DB_PATH=/data/mullet.db
ENV UPLOADS_DIR=/data/uploads
ENV LOG_PATH=/data/logs/mullet.log
ENV STATIC_DIR=/app/web/dist

EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["./server"]
