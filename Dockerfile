FROM golang:1-alpine AS builder

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# Install build tools for CGO
RUN apk add --no-cache build-base

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY docs ./docs/
COPY domain ./domain/
COPY internal ./internal/
COPY *.go .

RUN go build -o main .

FROM alpine:latest AS release

# Install runtime dependencies (certificates, timezone, init)
RUN apk add --no-cache ca-certificates tzdata dumb-init

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /build/main .
RUN mkdir -p /app/data
RUN chown -R appuser:appgroup /app/data

EXPOSE 3000
USER appuser

HEALTHCHECK CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:3000/_health || exit 1
ENTRYPOINT ["dumb-init", "--"]
CMD ["./main"]
