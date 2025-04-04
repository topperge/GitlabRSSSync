# Storage Options for GitlabRSSSync

GitlabRSSSync now supports two different storage backends for tracking which RSS items have been processed:

1. **Redis** (Original): In-memory data store with optional persistence
2. **SQLite** (New): File-based SQL database with optional S3 backup

This document outlines the advantages, configuration, and usage of each option.

## Redis Storage Backend

The original backend uses Redis to track processed RSS items.

### Advantages

- High performance for read/write operations
- Built-in support for set operations
- Support for distributed deployments via Redis Sentinel
- Well-tested in production

### Configuration

Set the following environment variables:

```
USE_SQLITE=false  # Default if not specified
REDIS_URL=redis:6379
REDIS_PASSWORD=  # Optional
USE_SENTINEL=false  # Optional, set to true for Redis Sentinel
```

## SQLite Storage Backend

The new SQLite backend uses a file-based SQL database to track processed RSS items.

### Advantages

- No separate database service required
- Simple deployment and maintenance
- Reduced resource usage
- Support for S3-compatible backup and restore
- Completely file-based, with all data in one file

### Configuration

Set the following environment variables:

```
USE_SQLITE=true
DB_PATH=/path/to/gitlabrsssync.db  # Default: $CONFIG_DIR/gitlabrsssync.db
```

### S3 Backup/Restore (Optional)

SQLite storage can be backed up to S3-compatible storage:

```
S3_ENABLED=true
S3_ENDPOINT=https://s3.amazonaws.com  # Optional, for non-AWS S3-compatible storage
S3_REGION=us-east-1
S3_BUCKET_NAME=your-bucket-name
S3_KEY_PREFIX=gitlabrsssync  # Optional, default: gitlabrsssync
S3_ACCESS_KEY=your-access-key  # Optional, can use instance profile instead
S3_SECRET_KEY=your-secret-key  # Optional, can use instance profile instead
S3_BACKUP_INTERVAL=6h  # Optional, default: 6 hours
```

## Usage and Migration

### Docker and Kubernetes

The Docker image contains both executables and automatically selects the appropriate one based on the `USE_SQLITE` environment variable.

### From Redis to SQLite

There's no automatic migration path from Redis to SQLite. If you want to switch:

1. Stop your GitlabRSSSync deployment
2. Set `USE_SQLITE=true` and other required environment variables
3. Start new deployment
4. Any items not yet in SQLite will be checked with GitLab to prevent duplicates

## Choosing the Right Backend

- **Use Redis** if:
  - You already have Redis infrastructure
  - You need high performance in large-scale deployments
  - You have a large number of feeds with frequent updates

- **Use SQLite** if:
  - You want a simpler deployment with fewer dependencies
  - You have a moderate amount of feeds
  - You want built-in backup/restore via S3
  - You want to minimize resource usage

## Implementation Details

The SQLite implementation provides:

- A `Store` interface for basic database operations
- Redis-compatible adapter for drop-in replacement
- Automatic table creation and indexing
- Backup and restore functionality with S3 support
- Prometheus metrics compatibility

## Monitoring and Health Checks

Both backends expose the same monitoring interfaces:

- Health check endpoint at `/healthz`
- Prometheus metrics at `/metrics`
