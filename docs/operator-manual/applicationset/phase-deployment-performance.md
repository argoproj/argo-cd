# Phase Deployment Performance Guidelines

This document provides performance optimization guidelines for ApplicationSet Phase Deployment at scale.

## Overview

Phase deployment can handle large-scale deployments efficiently with proper configuration and monitoring. This guide covers performance optimization strategies and best practices.

## Performance Features

### Concurrent Execution

The phase deployment implementation supports concurrent execution of hooks and checks:

- **Concurrent Phase Checks**: Up to 10 checks can run simultaneously
- **Concurrent Phase Hooks**: Up to 5 hooks can run simultaneously  
- **Configurable Concurrency**: Automatic adjustment based on the number of checks/hooks

### Resource Management

Built-in resource management features:

- **Memory Limits**: HTTP response bodies limited to 1MB
- **Timeout Controls**: Configurable timeouts prevent hanging operations
- **Context Cancellation**: Proper context handling for graceful shutdowns

### Scale Warnings

Automatic warnings for large-scale deployments:

- **Large Check Count**: Warns when phase has >20 checks
- **Large Hook Count**: Warns when phase has >10 hooks
- **Large Application Count**: Warns when phase targets >1000 applications

## Performance Optimization

### 1. Phase Design

#### Optimize Phase Structure
```yaml
# ✅ Good: Reasonable number of phases with clear boundaries
phases:
- name: "canary"
  percentage: 5
  checks:
  - name: "health-check"
    type: "http"
    http:
      url: "https://api.example.com/health"
- name: "production"
  percentage: 100
  checks:
  - name: "final-health-check"
    type: "http"
    http:
      url: "https://api.example.com/health"
```

#### Avoid Excessive Phases
```yaml
# ❌ Avoid: Too many phases can impact performance
phases:
- name: "phase-1"
  percentage: 1
- name: "phase-2" 
  percentage: 2
# ... 98 more phases (inefficient)
```

### 2. Check Optimization

#### Minimize Check Count per Phase
```yaml
# ✅ Good: Essential checks only
checks:
- name: "api-health"
  type: "http"
  http:
    url: "https://api.example.com/health"
    timeout: 30s
- name: "database-check"
  type: "command"
  command:
    command: ["pg_isready", "-h", "db.example.com"]
  timeout: 10s
```

#### Optimize Check Timeouts
```yaml
# ✅ Good: Appropriate timeouts
checks:
- name: "quick-health-check"
  type: "http"
  http:
    url: "https://api.example.com/ping"
  timeout: 5s  # Short timeout for simple checks
- name: "comprehensive-test"
  type: "command"
  command:
    command: ["integration-test.sh"]
  timeout: 300s  # Longer timeout for complex operations
```

### 3. Hook Optimization

#### Minimize Hook Execution Time
```yaml
# ✅ Good: Fast, specific operations
hooks:
- name: "notify-deployment"
  type: "http"
  http:
    url: "https://webhook.example.com/deploy"
    method: "POST"
    timeout: 10s
- name: "cache-warmup"
  type: "command"
  command:
    command: ["cache-warmer", "--quick"]
  timeout: 30s
```

#### Use Asynchronous Operations
```yaml
# ✅ Good: Trigger async operations
hooks:
- name: "trigger-tests"
  type: "http"
  http:
    url: "https://ci.example.com/trigger-tests"
    method: "POST"
    body: '{"async": true}'  # Don't wait for completion
```

### 4. Resource Configuration

#### Configure Controller Resources
```yaml
# ApplicationSet controller deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-applicationset-controller
spec:
  template:
    spec:
      containers:
      - name: argocd-applicationset-controller
        resources:
          limits:
            cpu: "2"      # Increase for large deployments
            memory: "2Gi" # Increase for many applications
          requests:
            cpu: "500m"
            memory: "512Mi"
```

#### Configure JVM Settings (if applicable)
```yaml
env:
- name: JAVA_OPTS
  value: "-Xmx1536m -XX:+UseG1GC -XX:MaxGCPauseMillis=100"
```

## Monitoring and Metrics

### Key Performance Metrics

Monitor these metrics for optimal performance:

1. **Phase Execution Time**: Total time per phase
2. **Check Duration**: Individual check execution times
3. **Hook Duration**: Individual hook execution times
4. **Concurrent Operations**: Number of simultaneous executions
5. **Resource Usage**: CPU and memory consumption

### Prometheus Metrics

#### Custom Metrics for Phase Deployment
```yaml
# Example metric definitions
- name: argocd_applicationset_phase_duration_seconds
  help: "Time spent executing phase deployment"
  type: histogram
  
- name: argocd_applicationset_check_duration_seconds
  help: "Time spent executing individual checks"
  type: histogram
  
- name: argocd_applicationset_hook_duration_seconds
  help: "Time spent executing individual hooks"
  type: histogram

- name: argocd_applicationset_concurrent_operations
  help: "Number of concurrent phase operations"
  type: gauge
```

#### Performance Monitoring Queries
```promql
# Average phase execution time
rate(argocd_applicationset_phase_duration_seconds_sum[5m]) / 
rate(argocd_applicationset_phase_duration_seconds_count[5m])

# 95th percentile check duration
histogram_quantile(0.95, rate(argocd_applicationset_check_duration_seconds_bucket[5m]))

# Hook failure rate
rate(argocd_applicationset_hook_failures_total[5m]) / 
rate(argocd_applicationset_hook_executions_total[5m])

# Resource utilization
container_memory_usage_bytes{container="argocd-applicationset-controller"} / 
container_spec_memory_limit_bytes{container="argocd-applicationset-controller"}
```

### Performance Dashboards

#### Grafana Dashboard Example
```json
{
  "dashboard": {
    "title": "ApplicationSet Phase Deployment Performance",
    "panels": [
      {
        "title": "Phase Execution Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(argocd_applicationset_phase_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      },
      {
        "title": "Concurrent Operations",
        "type": "graph", 
        "targets": [
          {
            "expr": "argocd_applicationset_concurrent_operations",
            "legendFormat": "Active operations"
          }
        ]
      }
    ]
  }
}
```

## Performance Tuning

### Large-Scale Deployments

For deployments with >500 applications:

1. **Increase Controller Resources**:
   ```yaml
   resources:
     limits:
       cpu: "4"
       memory: "4Gi"
   ```

2. **Optimize Phase Structure**:
   ```yaml
   # Use percentage-based phases for better distribution
   phases:
   - name: "canary"
     percentage: 5    # ~25 applications
   - name: "stage-1"
     percentage: 25   # ~125 applications  
   - name: "stage-2"
     percentage: 50   # ~250 applications
   - name: "production"
     percentage: 100  # All applications
   ```

3. **Reduce Check Frequency**:
   ```yaml
   # Use fewer, more efficient checks
   checks:
   - name: "critical-health-check"
     type: "http"
     http:
       url: "https://api.example.com/health"
       timeout: 10s
   # Remove non-essential checks
   ```

### Network Optimization

#### Optimize HTTP Checks
```yaml
# ✅ Good: Efficient HTTP configuration
http:
  url: "https://api.example.com/health"
  method: "HEAD"  # Lighter than GET for health checks
  timeout: 5s     # Short timeout
  expectedStatus: 200
  headers:
    "User-Agent": "ArgoCD-Health-Check"
  # Minimal headers for better performance
```

#### Connection Pooling
```yaml
# Configure HTTP client settings
env:
- name: HTTP_MAX_IDLE_CONNS
  value: "100"
- name: HTTP_MAX_CONNS_PER_HOST
  value: "10"
- name: HTTP_IDLE_CONN_TIMEOUT
  value: "30s"
```

## Troubleshooting Performance Issues

### Common Performance Problems

#### 1. Slow Phase Execution
**Symptoms**: Phases taking much longer than expected

**Diagnosis**:
```bash
# Check individual check durations
kubectl logs -l app.kubernetes.io/name=argocd-applicationset-controller | grep "Phase check"

# Monitor resource usage
kubectl top pods -l app.kubernetes.io/name=argocd-applicationset-controller
```

**Solutions**:
- Reduce check timeouts
- Optimize check commands/URLs
- Increase controller resources
- Reduce concurrent operations

#### 2. Memory Issues
**Symptoms**: Controller OOMKilled or high memory usage

**Diagnosis**:
```bash
# Check memory usage
kubectl describe pod -l app.kubernetes.io/name=argocd-applicationset-controller

# Review large HTTP responses
kubectl logs -l app.kubernetes.io/name=argocd-applicationset-controller | grep "responseSize"
```

**Solutions**:
- Increase memory limits
- Optimize HTTP response handling
- Reduce concurrent operations
- Split large phases into smaller ones

#### 3. High CPU Usage
**Symptoms**: Controller consuming excessive CPU

**Diagnosis**:
```bash
# Profile CPU usage
kubectl exec -it <controller-pod> -- pprof -http=:6060 /proc/self/profile
```

**Solutions**:
- Increase CPU limits
- Optimize command execution
- Reduce check frequency
- Use more efficient commands

## Best Practices Summary

### Configuration Best Practices

1. **Phase Design**:
   - Use 3-5 phases maximum for most deployments
   - Implement progressive rollout (5%, 25%, 50%, 100%)
   - Keep phases focused and purposeful

2. **Check Configuration**:
   - Limit to 5 essential checks per phase
   - Use appropriate timeouts (5-30 seconds for most checks)
   - Prefer HTTP HEAD requests for simple health checks

3. **Hook Configuration**:
   - Limit to 3 hooks per phase
   - Use asynchronous operations when possible
   - Keep hook execution time under 60 seconds

4. **Resource Management**:
   - Set appropriate resource limits
   - Monitor resource usage
   - Scale controller resources based on deployment size

### Monitoring Best Practices

1. **Essential Metrics**:
   - Phase execution duration
   - Check/hook success rates
   - Resource utilization
   - Concurrent operation count

2. **Alerting**:
   - Alert on phase execution timeouts
   - Monitor memory/CPU usage
   - Track check/hook failure rates

3. **Performance Testing**:
   - Test with realistic application counts
   - Validate performance under load
   - Monitor during peak deployment times

By following these performance guidelines, you can achieve efficient and scalable phase deployments that maintain system stability and performance.