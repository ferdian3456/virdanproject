# Build stage
FROM golang:1.23-alpine AS builder

# Install git and certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy binary from builder stage
COPY --from=builder /app/main .

# Copy .env file if it exists
COPY --from=builder /app/.env* ./

# Expose port
EXPOSE 8080

# Command to run the application
CMD ["./main"]