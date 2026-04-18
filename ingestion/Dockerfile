<<<<<<< HEAD
FROM golang:1.26-alpine AS builder
=======
FROM golang:1.22-alpine AS builder
>>>>>>> ts-notifications-read-api
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o ingestion ./ingestion

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/ingestion .
EXPOSE 3000
CMD ["./ingestion"]