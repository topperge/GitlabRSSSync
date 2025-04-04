#!/bin/sh

# Script to decide which binary to run based on environment variables

# Default configuration
DEFAULT_REDIS_BINARY="./rss_sync"
DEFAULT_SQLITE_BINARY="./rss_sync_sqlite"

# Set default data directory for SQLite
: "${DB_PATH:=/app/data/gitlabrsssync.db}"
export DB_PATH

# Log the selected mode
if [ "$USE_SQLITE" = "true" ]; then
    echo "Starting GitlabRSSSync with SQLite backend"
    echo "Using database file: $DB_PATH"
    
    # Check for S3 backup configuration
    if [ "$S3_ENABLED" = "true" ]; then
        echo "S3 backup enabled"
        echo "  Bucket: $S3_BUCKET_NAME"
        echo "  Key prefix: ${S3_KEY_PREFIX:-gitlabrsssync}"
        echo "  Region: ${S3_REGION:-us-east-1}"
    fi
    
    # Execute the SQLite version
    exec "$DEFAULT_SQLITE_BINARY"
else
    echo "Starting GitlabRSSSync with Redis backend"
    echo "Using Redis at: ${REDIS_URL:-localhost:6379}"
    
    # Execute the Redis version
    exec "$DEFAULT_REDIS_BINARY"
fi
