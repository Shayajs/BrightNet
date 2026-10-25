FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /brightnet-agent ./apps/agent/cmd/brightnet-agent

FROM debian:bookworm-slim
COPY --from=build /brightnet-agent /usr/local/bin/brightnet-agent
# Le processus reste root dans cette image de développement pour lire le socket Docker monté en lecture seule.
USER root
ENTRYPOINT ["brightnet-agent"]
