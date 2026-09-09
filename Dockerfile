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

ENV PORT=8080
ENV DB_PATH=/data/mullet.db
ENV UPLOADS_DIR=/data/uploads
ENV STATIC_DIR=/app/web/dist

EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["./server"]
