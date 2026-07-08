FROM golang:alpine AS builder

WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies.
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/compliance-server ./cmd/server

# Start a new stage from scratch
FROM alpine:latest

WORKDIR /app

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/compliance-server .

# Expose ports
EXPOSE 3000
EXPOSE 50051

# Command to run the executable
CMD ["./compliance-server"]
