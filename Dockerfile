# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /pingmesh ./cmd/pingmesh

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -h /var/lib/pingmesh pingmesh

COPY --from=builder /pingmesh /usr/local/bin/pingmesh

USER pingmesh
WORKDIR /var/lib/pingmesh

ENTRYPOINT ["pingmesh"]
CMD ["agent"]
