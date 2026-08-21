# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o migrate ./cmd/migrate

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Copy binaries from builder
COPY --from=builder /app/server .
COPY --from=builder /app/migrate .

# Cloud Run uses PORT env var
ENV PORT=8080
ENV GIN_MODE=release

EXPOSE 8080

# Run the binary
CMD ["./server"]