# GitlabRSSSync

An application that syncs RSS feeds to GitLab issues. It monitors RSS feeds and creates GitLab issues for new items, making it easy to track updates from various sources directly in your GitLab workflow.

![GitLabRSSSync Architecture](screenshots/GKEReleaseNotes.png)

## Overview

GitlabRSSSync solves the problem of keeping track of updates from multiple RSS sources by:
- Monitoring configured RSS feeds at regular intervals
- Checking for new entries against a Redis database
- Creating GitLab issues for new items with appropriate labels
- Supporting retroactive issue creation with preserved timestamps

Key features:
- Multiple feed support with per-feed configuration
- Intelligent duplicate detection
- Redis-based state tracking
- Prometheus metrics for monitoring
- Health check endpoint for Kubernetes deployments
- Redis Sentinel support for high availability

## Modernization Updates

This project has been modernized with:
- Updated Go version to 1.22 (from non-existent Go 1.23)
- Updated Redis client to v9 with context support
- Updated YAML parser to v3
- Enhanced UBI9 compatibility in the Dockerfile
- Added comprehensive test suite
- Implemented GitLab CI/CD pipeline with AutoDevOps
- Added Helm chart for Kubernetes deployment

## Development

### Prerequisites

- Go 1.22+
- Redis
- GitLab personal access token with API scope

### Local Setup

1. Clone the repository
2. Copy `config.yaml.example` to `config.yaml` and customize
3. Set environment variables:
   ```
   export GITLAB_API_BASE_URL="https://gitlab.com/api/v4"
   export GITLAB_API_TOKEN="your-gitlab-token"
   export CONFIG_DIR="/path/to/config/dir"
   export REDIS_URL="localhost:6379"
   export REDIS_PASSWORD=""
   ```
4. Run `go run main.go`

### Testing

Run the tests with:

```
go test -v ./...
```

For test coverage:

```
go test -cover ./...
```

## Docker

Build and run with Docker:

```
docker build -t gitlabrsssync .
docker run -e GITLAB_API_BASE_URL=https://gitlab.com/api/v4 \
           -e GITLAB_API_TOKEN=your-token \
           -e CONFIG_DIR=/app \
           -e REDIS_URL=redis:6379 \
           -e REDIS_PASSWORD="" \
           -v /path/to/config:/app \
           gitlabrsssync
```

## CI/CD Pipeline

The project includes a GitLab CI/CD pipeline configuration with:
- Testing with code coverage reporting
- Code linting
- Docker image building and publishing
- Automatic deployment to Kubernetes

### Pipeline Setup

1. Configure GitLab CI/CD variables:
   - `GITLAB_API_TOKEN`: Your GitLab API token
   - `KUBE_NAMESPACE`: Kubernetes namespace for deployment
   - `KUBE_INGRESS_BASE_DOMAIN`: Base domain for Ingress

2. Set up a Kubernetes cluster in GitLab
   - Go to your project's Operations > Kubernetes
   - Add a Kubernetes cluster
   - Enable Gitlab-managed cluster

### Pipeline Stages

- **Test**: Runs unit tests and linting
- **Build**: Builds the application and Docker image
- **Review**: Deploys to a review environment for feature branches
- **Deploy**: Deploys to production (manual trigger)

## Helm Chart

The included Helm chart deploys:
- The application with configurable resources
- Redis dependency for data storage
- Persistent volume for configuration

### Chart Configuration

Key values that can be configured:
- `replicaCount`: Number of application instances
- `image.repository`: Docker image repository
- `image.tag`: Docker image tag
- `env`: Environment variables for the application
- `persistence`: Storage configuration for persistent data
- `redis`: Redis service configuration

### Installing the Chart

Using Helm:

```
helm dependency update ./chart
helm install gitlab-rss-sync ./chart \
  --set env.GITLAB_API_TOKEN=your-token \
  --set env.GITLAB_API_BASE_URL=https://gitlab.com/api/v4
```

## Maintenance

To keep this application maintained:

1. Regularly update dependencies with:
   ```
   go get -u ./...
   go mod tidy
   ```

2. Monitor GitLab API changes that might affect the integration
3. Run the test suite to ensure compatibility
4. Update Helm chart versions for dependencies
