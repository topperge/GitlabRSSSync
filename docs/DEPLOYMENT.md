# Deployment Guide for GitlabRSSSync

This document details the various deployment options for the GitlabRSSSync application.

## Prerequisites

Regardless of deployment method, you'll need:

1. A GitLab account with API access
2. A Personal Access Token with appropriate permissions (`api` scope)
3. Redis instance (standalone or with Sentinel)
4. Configuration file (based on `config.yaml.example`)

## Local Deployment

### Running Directly

1. Install Go 1.22 or later
2. Clone the repository
3. Create your configuration:
   ```bash
   cp config.yaml.example config.yaml
   # Edit config.yaml with your feeds
   ```
4. Set up environment variables:
   ```bash
   export GITLAB_API_BASE_URL="https://gitlab.com/api/v4"
   export GITLAB_API_TOKEN="your-gitlab-token"
   export CONFIG_DIR="$(pwd)"
   export REDIS_URL="localhost:6379"
   export REDIS_PASSWORD=""
   ```
5. Run the application:
   ```bash
   go run main.go
   ```

### Running with Docker

1. Build the Docker image:
   ```bash
   docker build -t gitlabrsssync .
   ```

2. Run with Docker:
   ```bash
   docker run -d \
     -e GITLAB_API_BASE_URL="https://gitlab.com/api/v4" \
     -e GITLAB_API_TOKEN="your-gitlab-token" \
     -e CONFIG_DIR="/app" \
     -e REDIS_URL="redis:6379" \
     -e REDIS_PASSWORD="" \
     -v /path/to/your/config:/app \
     --name gitlabrsssync \
     gitlabrsssync
   ```

3. To use with Docker Compose, create a `docker-compose.yaml` file:
   ```yaml
   version: '3'
   services:
     gitlabrsssync:
       build: .
       environment:
         - GITLAB_API_BASE_URL=https://gitlab.com/api/v4
         - GITLAB_API_TOKEN=your-gitlab-token
         - CONFIG_DIR=/app
         - REDIS_URL=redis:6379
         - REDIS_PASSWORD=
       volumes:
         - ./config.yaml:/app/config.yaml
       depends_on:
         - redis
     
     redis:
       image: redis:latest
       volumes:
         - redis-data:/data
   
   volumes:
     redis-data:
   ```

4. Start the services:
   ```bash
   docker-compose up -d
   ```

## Kubernetes Deployment

### Using Helm Chart

1. Configure values:
   
   Create a `values-custom.yaml` file:
   ```yaml
   replicaCount: 1
   
   env:
     GITLAB_API_BASE_URL: "https://gitlab.com/api/v4"
     CONFIG_DIR: "/app/config"
     REDIS_URL: "gitlab-rss-sync-redis:6379"
     REDIS_PASSWORD: ""
   
   # Configure secrets
   secrets:
     enabled: true
     items:
       - key: GITLAB_API_TOKEN
         value: "your-gitlab-token"  # Replace with your token
   
   # Use persistent storage for config
   persistence:
     enabled: true
     size: 1Gi
   
   # Configure Redis
   redis:
     enabled: true
     architecture: standalone
   ```

2. Install the chart:
   ```bash
   # Update dependencies
   helm dependency update ./chart
   
   # Install the chart
   helm install gitlab-rss-sync ./chart -f values-custom.yaml
   ```

3. Create configuration:
   ```bash
   # Get the pod name
   POD_NAME=$(kubectl get pods -l app.kubernetes.io/name=gitlab-rss-sync -o jsonpath="{.items[0].metadata.name}")
   
   # Create config.yaml in the pod
   kubectl cp config.yaml $POD_NAME:/app/config/config.yaml
   ```

4. Check the deployment:
   ```bash
   kubectl get pods
   kubectl logs $POD_NAME
   ```

### Using GitLab AutoDevOps

The project includes GitLab CI/CD configuration for AutoDevOps:

1. Configure CI/CD variables in your GitLab project:
   - `GITLAB_API_TOKEN`: Your GitLab API token
   - `KUBE_NAMESPACE`: Kubernetes namespace
   - `KUBE_INGRESS_BASE_DOMAIN`: Base domain for ingress

2. Connect your Kubernetes cluster to GitLab:
   - Go to Operations > Kubernetes in your GitLab project
   - Add or connect a cluster
   - Install GitLab-managed apps

3. Push changes to your repository to trigger the pipeline

4. The Auto Deploy stage will deploy your application to the connected cluster

## High Availability Configuration

For production deployments, consider these high-availability options:

### Redis Sentinel

1. Set the `USE_SENTINEL` environment variable to enable Redis Sentinel mode:
   ```bash
   export USE_SENTINEL=true
   ```

2. Update your Redis URL to point to your Sentinel instances:
   ```bash
   export REDIS_URL="sentinel-host:26379"
   ```

3. Ensure your Redis configuration uses "mymaster" as the master name (or update the code)

### Horizontal Scaling

When deployed with Kubernetes, you can scale horizontally:

```bash
# Scale to 3 replicas
kubectl scale deployment gitlab-rss-sync --replicas=3
```

Or update your Helm values:
```yaml
replicaCount: 3
```

## Monitoring

GitlabRSSSync exposes Prometheus metrics at `/metrics` and a health check at `/healthz`.

### Prometheus Integration

1. Configure Prometheus to scrape the `/metrics` endpoint:
   ```yaml
   scrape_configs:
     - job_name: 'gitlab-rss-sync'
       static_configs:
         - targets: ['gitlab-rss-sync:8080']
   ```

2. Key metrics to monitor:
   - `last_run_time`
   - `issue_creation_total`
   - `issue_creation_error_total`

### Health Checks

The `/healthz` endpoint returns:
- HTTP 200 if Redis connection is working
- HTTP 500 if Redis connection fails

## Troubleshooting

### Common Issues

1. **Redis Connection Failed**
   - Check Redis URL and password
   - Ensure network connectivity between application and Redis
   - Verify Redis server is running

2. **GitLab API Authentication Failure**
   - Verify token has not expired
   - Check token has appropriate permissions
   - Ensure API URL is correct

3. **No Issues Created**
   - Check feed URLs are accessible
   - Verify feed items are newer than the `added_since` date
   - Check GitLab project ID is correct
   - Look for error messages in logs

### Logs

Examine logs for troubleshooting:

```bash
# Docker
docker logs gitlabrsssync

# Kubernetes
kubectl logs deployment/gitlab-rss-sync
```
