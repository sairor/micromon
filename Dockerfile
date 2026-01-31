# Build Stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy Go Mod and Sum
COPY go.mod go.sum ./
RUN go mod download

# Copy Source
COPY . .

# Build
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o mikromon ./cmd/server/main.go

# Production Stage
FROM alpine:latest

WORKDIR /app

# Install minimal dependencies (ca-certificates for SSL)
RUN apk --no-cache add ca-certificates tzdata

# Copy Binary
COPY --from=builder /app/mikromon .

# Expose HTTP and Syslog UDP
EXPOSE 8080
EXPOSE 514/udp

# Create data directory for JSON persistence fallback
RUN mkdir -p /app/data

# Run
CMD ["./mikromon"]
