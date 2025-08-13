# Sync Retry Options

Argo CD provides configurable retry mechanisms to handle sync failures gracefully. When enabled, retries can help resolve transient issues automatically, reducing manual intervention and improving application reliability. This document explains how retry options work, their configuration, and behavior in different scenarios.

## Overview

Sync retry options in Argo CD control whether failed sync operations are automatically retried. This feature is particularly useful in production environments where transient failures (such as network issues, temporary resource constraints, or cluster unavailability) can cause sync operations to fail.

By configuring retry options appropriately, you can:
- Automatically recover from transient failures
- Reduce manual intervention during sync operations
- Improve application deployment reliability
- Handle cluster resource constraints gracefully

## Retry Option States

### Retry OFF
- **Behavior**: No automatic retries on sync failures
- **Use Case**: When you want full control over sync operations
- **Timing**: Failed syncs wait until the next scheduled sync interval or manual sync
- **Best For**: Debugging sync issues, development environments, or when you need complete oversight

### Retry ON
- **Behavior**: Automatically retry failed sync operations
- **Use Case**: When you want to handle transient failures automatically
- **Timing**: Retries occur based on configured backoff settings until success or limit reached
- **Best For**: Production environments, high-availability requirements, or when minimizing manual intervention

## Sync Intervals and Retries

### Automated Sync
- **Default Interval**: 3 minutes
- **Behavior**: Argo CD checks Git repository for changes at regular intervals
- **With Retry OFF**: Failed syncs wait for next interval
- **With Retry ON**: Failed syncs retry automatically based on retry configuration

### Manual Sync
- **Trigger**: User-initiated sync operations
- **Behavior**: Immediate sync attempt regardless of retry settings
- **Use Case**: When you need immediate sync without waiting for intervals

## Configuration

### Basic Retry Configuration

Retry options are configured in the Application resource under `spec.syncPolicy.syncOptions` and `spec.retry`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-application
spec:
  project: default
  source:
    repoURL: https://github.com/my-repo.git
    path: manifests
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - Retry=true
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 1m
```

### Retry Parameters

| Parameter | Description | Default | Example | Required |
|-----------|-------------|---------|---------|----------|
| `limit` | Maximum number of retry attempts | - | `5` | Yes |
| `duration` | Initial retry interval | - | `5s` | Yes |
| `factor` | Exponential backoff multiplier | - | `2` | Yes |
| `maxDuration` | Maximum retry interval | - | `1m` | Yes |

### Advanced Retry Configuration

For more complex scenarios, you can configure different retry strategies:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: production-app
spec:
  # ... other specs ...
  retry:
    limit: 10
    backoff:
      duration: 10s
      factor: 1.5
      maxDuration: 5m
```

### Per-Resource Retry Configuration

You can also configure retry options for specific resources using annotations:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
  annotations:
    argocd.argoproj.io/sync-options: Retry=true
spec:
  # ... deployment spec ...
```

## Exponential Backoff

The `factor` parameter implements exponential backoff, where each retry interval increases by multiplying the previous interval by the factor. This approach helps prevent overwhelming the system with rapid retry attempts.

### Example Calculation

Using the configuration above:
- **Initial retry**: 5 seconds
- **Retry 1**: 5s × 2 = 10 seconds
- **Retry 2**: 10s × 2 = 20 seconds
- **Retry 3**: 20s × 2 = 40 seconds
- **Retry 4**: 40s × 2 = 80 seconds (capped at 60s by maxDuration)
- **Retry 5**: 60 seconds (maxDuration)

### Visual Representation

```
Retry Timeline:
  |
  |-- Attempt 1: Wait 5 seconds
  |
  |-- Attempt 2: Wait 10 seconds
  |
  |-- Attempt 3: Wait 20 seconds
  |
  |-- Attempt 4: Wait 40 seconds
  |
  |-- Attempt 5: Wait 60 seconds (capped)
```

### Backoff Factor Examples

Different factor values produce different retry patterns:

```yaml
# Linear backoff (factor: 1)
retry:
  limit: 5
  backoff:
    duration: 10s
    factor: 1
    maxDuration: 2m
# Results: 10s, 10s, 10s, 10s, 10s

# Moderate exponential backoff (factor: 1.5)
retry:
  limit: 5
  backoff:
    duration: 10s
    factor: 1.5
    maxDuration: 2m
# Results: 10s, 15s, 22.5s, 33.75s, 50.625s

# Aggressive exponential backoff (factor: 3)
retry:
  limit: 5
  backoff:
    duration: 10s
    factor: 3
    maxDuration: 2m
# Results: 10s, 30s, 90s, 120s (capped), 120s (capped)
```

## Use Cases

### When to Enable Retries
- **Transient failures**: Network issues, temporary resource constraints
- **High availability**: Minimize manual intervention
- **Production environments**: Automatic recovery from common failures
- **Cluster maintenance**: Handle temporary cluster unavailability
- **Resource constraints**: Wait for resources to become available

### When to Disable Retries
- **Debugging**: When investigating sync issues
- **Resource constraints**: To prevent excessive retry attempts
- **Manual control**: When you want full oversight of sync operations
- **Development environments**: Where immediate feedback is preferred
- **Testing**: When you need to see failures immediately

## Best Practices

### Retry Configuration
1. **Set appropriate limits**: Balance between automatic recovery and resource usage
2. **Configure backoff**: Use exponential backoff to avoid overwhelming systems
3. **Monitor retry patterns**: Identify recurring issues that may need attention
4. **Test retry behavior**: Verify retry configuration in non-production environments

### Operational Considerations
1. **Resource monitoring**: Ensure retries don't exhaust cluster resources
2. **Alerting**: Set up alerts for applications that frequently retry
3. **Documentation**: Document retry policies for your team
4. **Review cycles**: Regularly review and adjust retry configurations

### Environment-Specific Settings
```yaml
# Development environment - quick retries
retry:
  limit: 3
  backoff:
    duration: 5s
    factor: 1.5
    maxDuration: 30s

# Production environment - conservative retries
retry:
  limit: 10
  backoff:
    duration: 30s
    factor: 2
    maxDuration: 10m
```

## Troubleshooting

### Common Issues

#### Excessive Retries
- **Symptoms**: Application stuck in retry loop
- **Causes**: Low retry limits, aggressive backoff settings
- **Solutions**: Increase retry limits, adjust backoff parameters

#### Long Recovery Times
- **Symptoms**: Applications take too long to recover
- **Causes**: High maxDuration, high factor values
- **Solutions**: Reduce maxDuration, lower factor values

#### Resource Exhaustion
- **Symptoms**: Cluster resources depleted during retries
- **Causes**: High retry limits, short intervals
- **Solutions**: Reduce retry limits, increase backoff duration

### Debugging Commands

#### Check Application Status
```bash
# Check application status
argocd app get <app-name>

# View sync history
argocd app history <app-name>

# Check sync status
argocd app sync-status <app-name>
```

#### Monitor Retry Behavior
```bash
# Watch application events
argocd app logs <app-name> --follow

# Check application events
kubectl get events --field-selector involvedObject.name=<app-name> -n argocd
```

#### Analyze Retry Patterns
```bash
# Get detailed application information
argocd app get <app-name> -o yaml

# Check application health
argocd app health <app-name>
```

### Common Error Scenarios

#### Network Failures
```yaml
# Recommended retry configuration for network issues
retry:
  limit: 5
  backoff:
    duration: 10s
    factor: 2
    maxDuration: 2m
```

#### Resource Constraints
```yaml
# Recommended retry configuration for resource issues
retry:
  limit: 8
  backoff:
    duration: 30s
    factor: 1.5
    maxDuration: 5m
```

#### Cluster Maintenance
```yaml
# Recommended retry configuration for maintenance windows
retry:
  limit: 12
  backoff:
    duration: 1m
    factor: 2
    maxDuration: 15m
```

## Monitoring and Observability

### Metrics to Track
- **Retry count**: Number of retry attempts per application
- **Retry duration**: Time spent in retry loops
- **Success rate**: Percentage of successful syncs after retries
- **Failure patterns**: Common causes of sync failures

### Dashboard Examples
```yaml
# Grafana dashboard query example
# Retry attempts per application
argocd_app_sync_total{phase="Failed"} / argocd_app_sync_total * 100

# Average retry duration
histogram_quantile(0.95, argocd_app_sync_duration_seconds)
```

## Integration with Other Features

### Sync Windows
Retry options work in conjunction with sync windows. If a sync fails during a sync window, retries will respect the window configuration.

### Resource Hooks
Retry behavior applies to resource hooks as well. Failed hooks will be retried according to the retry configuration.

### Application Sets
Retry options can be configured at the ApplicationSet level and inherited by individual applications.

## Migration and Upgrades

### Enabling Retries on Existing Applications
```bash
# Enable retries for an existing application
argocd app set <app-name> --sync-option Retry=true

# Configure retry parameters
argocd app patch <app-name> --type merge -p '{"spec":{"retry":{"limit":5,"backoff":{"duration":"5s","factor":2,"maxDuration":"1m"}}}}'
```

### Disabling Retries
```bash
# Disable retries for an application
argocd app set <app-name> --sync-option Retry=false
```

## Conclusion

Argo CD retry options provide flexible control over sync failure handling. By understanding and configuring these options appropriately, you can balance automation with control, ensuring reliable application deployments while maintaining operational oversight.

Choose retry settings based on your operational requirements, failure patterns, and resource constraints. Regular monitoring and adjustment of retry configurations will help optimize your Argo CD deployment for your specific use case.

Remember that retry options are just one part of a comprehensive application deployment strategy. Combine them with other Argo CD features like sync windows, resource hooks, and proper monitoring to create robust, reliable deployment pipelines.
