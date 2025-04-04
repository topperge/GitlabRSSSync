# GitlabRSSSync Architecture

This document provides an overview of the GitlabRSSSync application architecture and implementation details.

## System Architecture

GitlabRSSSync is designed with a simple but effective architecture:

```
                                    ┌─────────────┐
                                    │             │
                ┌─────────────────►│  RSS Feeds  │
                │                   │             │
                │                   └─────────────┘
                │                           │
                │                           ▼
┌───────────────┴───────┐           ┌─────────────┐
│                       │           │             │
│    RSS Gitlab Sync    │◄─────────►│    Redis    │
│                       │           │             │
└───────────────┬───────┘           └─────────────┘
                │                           ▲
                │                           │
                │                   ┌─────────────┐
                │                   │             │
                └─────────────────►│   GitLab    │
                                    │             │
                                    └─────────────┘
```

### Core Components

1. **Main Application**: A Go service that orchestrates the entire process
2. **Redis Database**: Stores feed item GUIDs to track processed items
3. **RSS Feed Parser**: Fetches and parses RSS feeds
4. **GitLab API Client**: Creates issues in GitLab

## Implementation Details

### Main Process Flow

1. The application starts and initializes:
   - Prometheus metrics
   - GitLab client
   - Redis client
   - Configuration from YAML file

2. For each configured feed, at regular intervals:
   - Parse the RSS feed
   - Check each item against Redis to determine if it's new
   - For new items, verify they don't already exist in GitLab
   - Create GitLab issues for new items
   - Store the item GUID in Redis to mark it as processed

### Data Structures

#### Config

```go
type Config struct {
    Feeds    []Feed
    Interval int
}

type Feed struct {
    ID              string
    FeedURL         string `yaml:"feed_url"`
    Name            string
    GitlabProjectID int    `yaml:"gitlab_project_id"`
    Labels          []string
    AddedSince      time.Time `yaml:"added_since"`
    Retroactive     bool
}
```

#### Environment Configuration

```go
type EnvValues struct {
    RedisURL         string
    RedisPassword    string
    ConfDir          string
    GitlabAPIKey     string
    GitlabAPIBaseUrl string
    UseSentinel      bool
}
```

### Key Functions

- `main()`: Entry point that sets up and initializes everything
- `initialise()`: Configures Prometheus metrics, clients, and reads configuration
- `readConfig()`: Parses YAML configuration
- `readEnv()`: Reads environment variables
- `checkFeed()`: Core logic for processing a feed
- `hasExistingGitlabIssue()`: Checks if an issue already exists in GitLab
- `checkLiveliness()`: Provides health check endpoint

### Metrics

GitlabRSSSync exposes the following Prometheus metrics:

- `last_run_time`: Timestamp of the last run time
- `issue_creation_total`: Count of issues created
- `issue_creation_error_total`: Count of issue creation errors

## Redis Usage

GitlabRSSSync uses Redis as a persistence layer to track processed items:

- **Key Structure**: Each feed has a key identified by its `ID`
- **Value Structure**: Set of GUIDs for items that have been processed
- **Operations**:
  - `SIsMember`: Check if an item GUID exists in the set
  - `SAdd`: Add a GUID to the set

## High Availability

The application supports Redis Sentinel for high availability:

- Enable with the `USE_SENTINEL` environment variable
- Failover handled by the Redis client

## Health Checks

A `/healthz` endpoint is provided to verify:
- Application is running
- Redis connection is working

## Security Considerations

- All sensitive information (API tokens, passwords) is provided via environment variables
- No credentials are logged
- Static build with minimal dependencies reduces attack surface

## Kubernetes Deployment

When deployed on Kubernetes:
- Horizontal scaling can be applied
- Persistent volume for configuration
- Secrets for sensitive values
- Health checks for liveness and readiness probes
- Redis can be deployed as a dependency or connected to an external service
