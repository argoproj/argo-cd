# Sync Retry Options

Argo CD provides configurable retry mechanisms to handle sync failures gracefully, automatically resolving transient issues and reducing manual intervention.

## Overview

Sync retry options control whether failed sync operations are automatically retried. Useful for production environments where transient failures (network issues, resource constraints, cluster unavailability) can cause sync failures.

Benefits:
- Automatic recovery from transient failures
- Reduced manual intervention
- Improved deployment reliability

## Retry Option States

### Retry OFF
- No automatic retries on sync failures
- Full control over sync operations
- Best for debugging, development, or complete oversight
- **Use cases**: Local testing, CI/CD pipelines, issue investigation, resource constraints

### Retry ON
- Automatic retry of failed sync operations
- Handles transient failures automatically
- Best for production, high-availability, or minimal manual intervention
- **Use cases**: High-availability apps, network issues, resource quotas, service dependencies

## Sync Behavior

- **Automated**: 3-minute intervals with retry behavior based on configuration
- **Manual**: Immediate sync regardless of retry settings
- **With retries**: Failed syncs retry automatically based on backoff settings
- **Without retries**: Failed syncs wait for next interval or manual sync

## Configuration

Configure retry options in `spec.syncPolicy`:

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
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 1m
```

### Retry Parameters

| Parameter | Description | Example | Required |
|-----------|-------------|---------|----------|
| `limit` | Maximum retry attempts | `5` | Yes |
| `duration` | Initial retry interval | `5s` | Yes |
| `factor` | Exponential backoff multiplier | `2` | Yes |
| `maxDuration` | Maximum retry interval | `1m` | Yes |

## Exponential Backoff

The `factor` parameter implements exponential backoff, where each retry interval increases by multiplying the previous interval by the factor. This approach helps prevent overwhelming the system with rapid retry attempts.

Example calculation using the configuration above:
- **Initial retry**: 5 seconds
- **Retry 1**: 5s × 2 = 10 seconds
- **Retry 2**: 10s × 2 = 20 seconds
- **Retry 3**: 20s × 2 = 40 seconds
- **Retry 4**: 40s × 2 = 80 seconds (capped at 60s by maxDuration)
- **Retry 5**: 60 seconds (maxDuration)

## Best Practices

1. **Set appropriate limits**: Balance recovery vs. resource usage
2. **Use exponential backoff**: Prevent system overwhelming
3. **Monitor patterns**: Identify recurring issues
4. **Monitor resources**: Ensure retries don't exhaust cluster
5. **Set up alerting**: For frequently retrying applications

## Troubleshooting

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Excessive retries | Low limits, aggressive backoff | Increase limits, adjust backoff |
| Long recovery | High maxDuration/factor | Reduce maxDuration, lower factor |
| Resource exhaustion | High limits, short intervals | Reduce limits, increase duration |

## Enable Retries

```bash
argocd app patch <app-name> --type merge -p '{"spec":{"retry":{"limit":5,"backoff":{"duration":"5s","factor":2,"maxDuration":"1m"}}}}'
```
