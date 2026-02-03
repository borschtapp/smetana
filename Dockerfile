FROM golang:1-alpine AS builder

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# Install build tools for CGO
RUN apk add --no-cache build-base

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

FROM alpine:latest AS release

# Install runtime dependencies (certificates, timezone, init)
RUN apk add --no-cache ca-certificates tzdata dumb-init

WORKDIR /app
COPY --from=builder /build/main .
RUN mkdir -p /app/data

EXPOSE 3000
HEALTHCHECK CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:3000/_health || exit 1
ENTRYPOINT ["/usr/bin/dumb-init", "--", "./main"]
