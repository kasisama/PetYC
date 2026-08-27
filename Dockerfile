# syntax=docker/dockerfile:1

FROM node:22-alpine AS admin-builder
WORKDIR /src

COPY admin-ui/package.json admin-ui/package-lock.json ./admin-ui/
RUN npm --prefix admin-ui ci

COPY admin-ui ./admin-ui
RUN npm --prefix admin-ui run build

FROM golang:1.26.4-alpine AS backend-builder
ARG VERSION=dev
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=admin-builder /src/admin/dist ./admin/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/petyc .

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 petyc \
    && adduser -S -D -H -u 10001 -G petyc petyc \
    && mkdir -p /app /data /config \
    && chown -R petyc:petyc /app /data /config

COPY --from=backend-builder --chown=petyc:petyc /out/petyc /app/petyc

USER petyc
WORKDIR /data
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/petyc"]
