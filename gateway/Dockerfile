# Build stage — compiles the Go binary
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy module files and download dependencies first
# This layer is cached unless go.mod or go.sum changes
COPY go.mod go.sum ./
RUN go mod download

# Copy the gateway source code and build
COPY gateway/ ./gateway/
RUN go build -o gateway ./gateway/

# Run stage — minimal image with just the binary
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/gateway .
EXPOSE 8080
CMD ["./gateway"]