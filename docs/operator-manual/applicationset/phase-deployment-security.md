# Phase Deployment Security Guidelines

This document provides security guidelines for using ApplicationSet Phase Deployment hooks and checks.

## Overview

Phase deployment hooks and checks execute in the ArgoCD ApplicationSet controller pod context and can run arbitrary commands or make HTTP requests. This document outlines security considerations and best practices.

## Security Features

### Command Security Validation

The phase deployment implementation includes built-in security validation for command execution:

- **Command Path Validation**: Warns about absolute path commands that may indicate path traversal attempts
- **Dangerous Command Detection**: Warns about potentially dangerous commands (rm, sudo, curl, etc.)
- **Environment Variable Limits**: Enforces maximum number of environment variables (100)
- **Sensitive Environment Variable Detection**: Warns about potentially sensitive environment variable names

### HTTP Security Validation

HTTP checks and hooks include security validation:

- **URL Scheme Validation**: Only HTTP and HTTPS schemes are supported
- **Internal Network Detection**: Warns about requests to localhost/private IP ranges
- **Header Limits**: Enforces maximum number of HTTP headers (50)
- **Sensitive Header Detection**: Warns about potentially sensitive headers (Authorization, Cookie, etc.)
- **Response Size Limits**: Response bodies are limited to 1MB to prevent memory exhaustion

### Timeout Validation

All commands and HTTP requests have configurable timeouts with maximum limits:

- **Command Timeout**: Maximum 30 minutes
- **HTTP Timeout**: Maximum 10 minutes
- **Check Timeout**: Configurable per check
- **Hook Timeout**: Configurable per hook

## Security Best Practices

### 1. Command Execution Security

#### Use Specific Commands
```yaml
command:
  command: ["/usr/local/bin/health-check", "--endpoint", "http://api.example.com"]
```

#### Avoid Shell Injection
❌ **Dangerous:**
```yaml
command:
  command: ["sh", "-c", "curl $ENDPOINT"]  # Shell injection risk
```

✅ **Safe:**
```yaml
command:
  command: ["curl", "--fail", "--silent", "http://api.example.com/health"]
```

#### Limit Environment Variables
```yaml
command:
  command: ["health-check"]
  env:
    CHECK_ENDPOINT: "http://api.example.com"
    TIMEOUT: "30s"
    # Keep environment variables minimal and specific
```

### 2. HTTP Security

#### Use HTTPS When Possible
```yaml
http:
  url: "https://api.example.com/health"  # Prefer HTTPS
  method: "GET"
  expectedStatus: 200
```

#### Validate Certificates
```yaml
http:
  url: "https://api.example.com/health"
  insecureSkipVerify: false  # Default: verify certificates
```

#### Limit Headers
```yaml
http:
  url: "https://api.example.com/health"
  headers:
    "Accept": "application/json"
    "User-Agent": "ArgoCD-Health-Check"
    # Avoid sensitive headers in plain text
```

### 3. Authentication and Authorization

#### Use Kubernetes Secrets
```yaml
# Store sensitive data in Kubernetes secrets
apiVersion: v1
kind: Secret
metadata:
  name: api-credentials
type: Opaque
data:
  token: <base64-encoded-token>
```

#### Reference Secrets in Environment Variables
```yaml
command:
  command: ["health-check"]
  env:
    API_TOKEN_FILE: "/var/secrets/api-token"
```

### 4. Network Security

#### Restrict Network Access
- Use NetworkPolicies to limit outbound traffic from the ApplicationSet controller
- Consider using a service mesh for secure communication
- Validate destination URLs and IP ranges

#### Example NetworkPolicy
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: applicationset-controller-egress
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: argocd-applicationset-controller
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: argocd
    ports:
    - protocol: TCP
      port: 443
  - to: []  # Allow specific external endpoints
    ports:
    - protocol: TCP
      port: 443
```

### 5. Resource Limits

#### Configure Resource Constraints
```yaml
# In ApplicationSet controller deployment
resources:
  limits:
    cpu: "1"
    memory: "512Mi"
  requests:
    cpu: "100m"
    memory: "128Mi"
```

#### Monitor Resource Usage
- Monitor CPU and memory usage during phase deployments
- Set up alerts for unusual resource consumption
- Use monitoring tools to track hook execution times

## Security Configurations

### Version Compatibility

Ensure compatibility by adding version annotations:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: my-app-set
  annotations:
    argocd.argoproj.io/min-version: "v2.12.0"  # Phase deployment minimum version
spec:
  # ApplicationSet configuration
```

### Performance and Security Limits

The implementation enforces the following limits:

| Setting | Default Limit | Purpose |
|---------|---------------|---------|
| Concurrent Checks | 10 | Prevent resource exhaustion |
| Concurrent Hooks | 5 | Limit parallel execution |
| Applications per Phase | 1000 | Performance optimization |
| Command Timeout | 30 minutes | Prevent hanging processes |
| HTTP Timeout | 10 minutes | Prevent hanging requests |
| Environment Variables | 100 | Limit configuration complexity |
| HTTP Headers | 50 | Prevent excessive headers |
| Response Body Size | 1MB | Prevent memory exhaustion |

## Monitoring and Alerting

### Security Monitoring

Monitor the following security events:

1. **Command Security Warnings**: Commands flagged as potentially dangerous
2. **HTTP Security Warnings**: Requests to internal networks or insecure endpoints
3. **Timeout Violations**: Commands or HTTP requests exceeding time limits
4. **Resource Usage**: Unusual CPU or memory consumption

### Example Monitoring Queries

#### Prometheus Queries
```promql
# Count security warnings
increase(argocd_applicationset_phase_security_warnings_total[5m])

# Monitor hook execution times
histogram_quantile(0.95, rate(argocd_applicationset_hook_duration_seconds_bucket[5m]))

# Track timeout failures
increase(argocd_applicationset_timeout_failures_total[5m])
```

#### Log-based Monitoring
```yaml
# Example log alert configuration
- alert: PhaseDeploymentSecurityWarning
  expr: rate(log_messages{level="warn", component="phase-deployment"}[5m]) > 0.1
  for: 1m
  annotations:
    summary: "Phase deployment security warnings detected"
    description: "Security warnings in phase deployment: {{ $labels.message }}"
```

## Incident Response

### Security Incident Checklist

1. **Immediate Response**:
   - Identify affected ApplicationSets
   - Review recent phase deployment logs
   - Check for unauthorized command execution
   - Verify HTTP request destinations

2. **Investigation**:
   - Analyze security warning logs
   - Review command and HTTP configurations
   - Check for configuration changes
   - Validate access controls

3. **Remediation**:
   - Update ApplicationSet configurations
   - Implement additional security controls
   - Update monitoring and alerting
   - Document lessons learned

### Emergency Procedures

#### Disable Phase Deployment
```bash
# Temporarily disable phase deployment for an ApplicationSet
kubectl patch applicationset <name> -p '{"spec":{"generators":[{"list":{"elements":[]}}]}}'
```

#### Stop Running Hooks
```bash
# Scale down ApplicationSet controller (emergency only)
kubectl scale deployment argocd-applicationset-controller --replicas=0
```

## Compliance and Auditing

### Audit Trail

Ensure comprehensive audit trails:

- Log all hook executions with timestamps
- Record command line arguments and environment variables
- Track HTTP request URLs and response codes
- Monitor resource usage and performance metrics

### Compliance Requirements

For compliance with security standards:

1. **Access Control**: Implement RBAC for ApplicationSet management
2. **Encryption**: Use HTTPS for external communications
3. **Secrets Management**: Store sensitive data in Kubernetes secrets
4. **Monitoring**: Implement comprehensive logging and monitoring
5. **Documentation**: Maintain security configuration documentation

## Conclusion

Phase deployment hooks and checks provide powerful automation capabilities but require careful security consideration. Follow these guidelines to implement secure phase deployments while maintaining operational efficiency.

For additional security questions or to report security issues, please follow the ArgoCD security reporting guidelines.