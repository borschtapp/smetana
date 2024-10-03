# Building the binary of the App
FROM golang:1.23 AS build

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR /build

# Copy all the Code and stuff to compile everything
COPY . .

# Downloads all the dependencies in advance (could be left out, but it's more clear this way)
RUN go mod download

# Builds the application as a staticly linked one, to allow it to run on alpine
RUN go build -a -installsuffix cgo -o main .

# Moving the binary to the 'final Image' to make it smaller
FROM alpine:latest as release

WORKDIR /app

# Create the `public` dir and copy all the assets into it
# RUN mkdir ./static
# COPY ./static ./static

COPY --from=build /build/main .
COPY --from=build /build/.env .
RUN chmod +x /app/main

RUN apk add --no-cache dumb-init ca-certificates libc6-compat

# Exposes port 3000 because our program listens on that port
EXPOSE 3000

ENTRYPOINT ["/usr/bin/dumb-init", "--"]