# ---- Build Stage ----
# Note: SQLite requires CGO to be enabled
FROM golang:1.22-alpine AS builder

# Install build dependencies for SQLite
RUN apk add --no-cache gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the application with SQLite support
# CGO_ENABLED=1 is required for SQLite
# We still use static linking where possible
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags="-w -s" -o rss_sync_sqlite ./cmd/sqlite

# We can also build the original Redis version if needed
# This version doesn't require CGO
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-w -s" -o rss_sync .

# Copy wait-for-it.sh script
COPY wait-for-it.sh .
RUN chmod +x ./wait-for-it.sh

# ---- Final Stage ----
# Use UBI 9 minimal as the final base image
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install required dependencies for UBI9
# We need SQLite for the SQLite version
RUN microdnf update -y && \
    microdnf install -y ca-certificates sqlite && \
    microdnf clean all

# Set working directory
WORKDIR /app

# Copy the compiled binaries from the builder stage
COPY --from=builder /app/rss_sync .
COPY --from=builder /app/rss_sync_sqlite .
# Copy the wait-for-it script from the builder stage
COPY --from=builder /app/wait-for-it.sh .

# Ensure the binaries and script are executable
RUN chmod +x ./rss_sync ./rss_sync_sqlite ./wait-for-it.sh

# Create a data directory for SQLite
RUN mkdir -p /app/data && chmod 777 /app/data

# Environment variable to select which version to run
ENV USE_SQLITE=false

# Set the entrypoint script
COPY --from=builder /app/entrypoint.sh .
RUN chmod +x ./entrypoint.sh

# The entrypoint script will decide which binary to run
CMD ["./entrypoint.sh"]
