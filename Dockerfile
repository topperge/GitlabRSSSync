# ---- Build Stage ----
# Use a specific Go version on Alpine for a smaller build image
FROM golang:1.22-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the application statically
# CGO_ENABLED=0 prevents linking against C libraries
# -ldflags="-w -s" strips debug information and symbol table for smaller binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-w -s" -o rss_sync .

# Copy wait-for-it.sh script
COPY wait-for-it.sh .
RUN chmod +x ./wait-for-it.sh

# ---- Final Stage ----
# Use UBI 9 minimal as the final base image
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install required dependencies for UBI9
RUN microdnf update -y && \
    microdnf install -y ca-certificates && \
    microdnf clean all

# Set working directory
WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/rss_sync .
# Copy the wait-for-it script from the builder stage
COPY --from=builder /app/wait-for-it.sh .

# Ensure the binary and script are executable
RUN chmod +x ./rss_sync ./wait-for-it.sh

# Set the command to run the application binary
# The Kubernetes deployment manifest will handle the wait-for-it.sh logic
CMD ["./rss_sync"]
