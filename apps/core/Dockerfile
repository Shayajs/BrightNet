FROM node:20-bookworm-slim AS web
WORKDIR /src
COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /brightnet-core ./apps/core/cmd/brightnet-core

FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates adduser \
  && adduser --system --uid 65532 --group brightnet \
  && mkdir -p /data && chown brightnet:brightnet /data \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /brightnet-core /usr/local/bin/brightnet-core
COPY --from=web /src/dist /usr/local/share/brightnet/web
COPY apps/core/docker-entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
EXPOSE 8787 1717 1718
ENTRYPOINT ["/entrypoint.sh"]
