# Generator Phase Deployment Strategy

The Phase Deployment Strategy enables ApplicationSet generators to deploy applications across multiple clusters in a controlled, phased approach. This feature allows you to:

- Deploy to different clusters in specific phases/stages
- Execute pre-deployment and post-deployment hooks
- Run validation checks between phases
- Automatically stop or rollback deployments when checks fail
- Control the number of applications deployed per phase

## Overview

Phase deployment strategy can be applied to any ApplicationSet generator (List, Cluster, Git, etc.) by adding a `deploymentStrategy` field to the generator configuration. The strategy defines multiple phases, each targeting specific clusters and including optional validation checks.

## Configuration

### Basic Structure

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: multi-phase-deployment
spec:
  generators:
  - clusters:
      selector:
        matchLabels:
          environment: "dev"
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: "dev-phase"
        targets:
        - clusters: ["dev-cluster-1", "dev-cluster-2"]
        checks:
        - name: "health-check"
          type: "http"
          http:
            url: "https://monitoring.example.com/health"
            expectedStatus: 200
      - name: "staging-phase"
        targets:
        - clusters: ["staging-cluster"]
        checks:
        - name: "integration-tests"
          type: "command"
          command:
            command: ["./run-integration-tests.sh"]
            env:
              ENVIRONMENT: "staging"
  template:
    metadata:
      name: "{{name}}"
    spec:
      project: "default"
      source:
        repoURL: https://github.com/example/manifests
        targetRevision: HEAD
        path: "{{path}}"
      destination:
        server: "{{server}}"
        namespace: "{{namespace}}"
```

## Phase Configuration

### Phase Deployment Methods

You can configure phases using two different approaches:

1. **Target-based deployment**: Deploy to specific clusters/environments in each phase
2. **Percentage-based deployment**: Deploy a percentage of all applications in each phase

### Phase Targets (Target-based Deployment)

Define which applications should be deployed in each phase using one of the following targeting methods:

#### 1. Cluster Targeting

```yaml
phases:
- name: "dev-phase"
  targets:
  - clusters: ["dev-cluster-1", "dev-cluster-2"]
```

#### 2. Value Matching

```yaml
phases:
- name: "environment-specific"
  targets:
  - values:
      environment: "development"
      region: "us-west-2"
```

#### 3. Match Expressions

```yaml
phases:
- name: "advanced-targeting"
  targets:
  - matchExpressions:
    - key: "environment"
      operator: "In"
      values: ["dev", "test"]
    - key: "region"
      operator: "NotIn"
      values: ["us-east-1"]
```

### Percentage-based Deployment

Deploy a specific percentage of all applications in each phase. This is ideal for canary deployments and gradual rollouts:

```yaml
phases:
- name: "canary"
  percentage: 10  # Deploy 10% of applications
  checks:
  - name: "canary-health"
    type: "http"
    http:
      url: "https://monitoring.example.com/canary-health"
  waitDuration: "10m"
  
- name: "partial-rollout"
  percentage: 40  # Deploy 40% more (50% total)
  checks:
  - name: "partial-health"
    type: "http"
    http:
      url: "https://monitoring.example.com/partial-health"
  waitDuration: "5m"
  
- name: "full-deployment"
  percentage: 50  # Deploy remaining 50% (100% total)
  checks:
  - name: "full-health"
    type: "http"
    http:
      url: "https://monitoring.example.com/full-health"
```

**Percentage Deployment Features:**

- **Cumulative**: Each phase percentage is cumulative (10% + 40% + 50% = 100%)
- **Ordered**: Applications are consistently ordered alphabetically by name for predictable deployments
- **Automatic calculation**: The system automatically calculates which applications belong to each phase
- **Minimum guarantee**: If percentage > 0, at least 1 application will be deployed even if calculation rounds to 0

#### Combining Percentage with MaxUpdate

You can combine percentage-based deployment with `maxUpdate` constraints:

```yaml
phases:
- name: "controlled-canary"
  percentage: 20  # Want to deploy 20% of applications
  maxUpdate: 2    # But never more than 2 at once
  checks:
  - name: "canary-check"
    type: "http"
    http:
      url: "https://health.example.com/canary"
```

In this case, if 20% would result in 5 applications, `maxUpdate: 2` limits it to only 2 applications per phase execution.

#### Combining Percentage with Cluster Filtering

You can combine percentage-based deployment with cluster targeting to control both WHICH clusters to deploy to AND what percentage of applications to deploy:

```yaml
phases:
- name: "dev-canary-20-percent"
  percentage: 20  # Deploy 20% of applications
  targets:
  - matchExpressions:
    - key: "environment"
      operator: "In"
      values: ["dev", "development"]
    - key: "region"
      operator: "In"
      values: ["us-west-2"]
  checks:
  - name: "dev-health"
    type: "http"
    http:
      url: "https://{{name}}.dev.us-west-2.example.com/health"
      
- name: "staging-gradual-30-percent"
  percentage: 30  # Deploy 30% more (50% total)
  targets:
  - matchExpressions:
    - key: "environment"
      operator: "In"
      values: ["staging"]
  maxUpdate: 3  # But never more than 3 at once
  
- name: "prod-canary-25-percent"
  percentage: 25  # Deploy 25% more (75% total)
  targets:
  - matchExpressions:
    - key: "environment"
      operator: "In"
      values: ["production"]
    - key: "tier"
      operator: "In"
      values: ["canary"]
      
- name: "prod-full-25-percent"
  percentage: 25  # Deploy remaining 25% (100% total)
  targets:
  - matchExpressions:
    - key: "environment"
      operator: "In"
      values: ["production"]
    # No tier restriction = all production clusters
```

This approach enables sophisticated deployment strategies like:
- **Environment-Aware Canaries**: Deploy percentages to specific environments first
- **Regional Rollouts**: Deploy percentages to specific regions progressively  
- **Tier-Based Deployment**: Deploy to canary tiers first, then general production
- **Risk-Controlled Rollouts**: Combine environment, region, and percentage controls

### Phase Controls

#### Max Update

Limit the number of applications deployed in a single phase:

```yaml
phases:
- name: "canary-phase"
  targets:
  - clusters: ["prod-cluster-1", "prod-cluster-2", "prod-cluster-3"]
  maxUpdate: 1  # Deploy to only 1 cluster at a time
```

#### Wait Duration

Add a delay between phases:

```yaml
phases:
- name: "staged-rollout"
  targets:
  - clusters: ["staging"]
  waitDuration: "5m"  # Wait 5 minutes before proceeding
```

## Pre and Post Deployment Hooks

Phase deployment supports pre-deployment and post-deployment hooks that execute at specific points in the deployment lifecycle. Unlike validation checks that run during phase execution, hooks run **before** and **after** the actual deployment occurs.

### Hook Types

#### Pre-deployment Hooks (`preHooks`)
Execute **before** applications are deployed in a phase. Useful for:
- Infrastructure validation
- Pre-deployment notifications  
- Resource preparation
- Security checks

#### Post-deployment Hooks (`postHooks`)
Execute **after** applications are deployed in a phase. Useful for:
- Deployment verification
- Success notifications
- Cleanup operations
- Metric collection

### Hook Configuration

```yaml
phases:
- name: "production-phase"
  targets:
  - clusters: ["prod-cluster"]
  preHooks:
  - name: "pre-deployment-notification"
    type: "http"
    failurePolicy: "fail"
    timeout: "30s"
    http:
      url: "https://notifications.example.com/webhook"
      method: "POST"
      headers:
        Content-Type: "application/json"
      body: |
        {
          "message": "Starting production deployment",
          "phase": "production-phase",
          "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
        }
  - name: "infrastructure-check"
    type: "command"
    failurePolicy: "fail"
    timeout: "5m"
    command:
      command: ["kubectl", "get", "nodes", "--no-headers"]
      env:
        KUBECONFIG: "/etc/kubeconfig"
  postHooks:
  - name: "deployment-verification"
    type: "command"
    failurePolicy: "ignore"
    timeout: "10m"
    command:
      command: ["./verify-deployment.sh"]
      env:
        ENVIRONMENT: "production"
  - name: "success-notification"
    type: "http"
    failurePolicy: "ignore"
    http:
      url: "https://notifications.example.com/webhook"
      method: "POST"
      headers:
        Content-Type: "application/json"
      body: |
        {
          "message": "Production deployment completed successfully",
          "phase": "production-phase"
        }
```

### Hook Failure Policies

Configure how the system responds to hook failures:

- **`fail`** (default): Stop the deployment if the hook fails
- **`ignore`**: Continue the deployment even if the hook fails (logs warning)
- **`abort`**: Immediately stop the entire deployment process

```yaml
preHooks:
- name: "critical-pre-check"
  type: "command"
  failurePolicy: "fail"    # Must succeed or deployment stops
  command:
    command: ["./critical-check.sh"]

- name: "optional-notification"
  type: "http"
  failurePolicy: "ignore"  # Failure won't block deployment
  http:
    url: "https://optional-webhook.example.com"

- name: "security-validation"
  type: "command"
  failurePolicy: "abort"   # Failure stops entire process
  command:
    command: ["./security-scan.sh"]
```

### Hook Environment Variables

The following environment variables are automatically available in command hooks:

- `APPSET_NAME`: Name of the ApplicationSet
- `APPSET_NAMESPACE`: Namespace of the ApplicationSet  
- `HOOK_NAME`: Name of the current hook
- `HOOK_TYPE`: Type of hook execution (`pre` or `post`)

### Complete Hook Example

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: deployment-with-hooks
spec:
  generators:
  - clusters:
      selector:
        matchLabels:
          environment: "production"
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: "canary"
        percentage: 10
        preHooks:
        - name: "canary-preparation"
          type: "command"
          failurePolicy: "fail"
          timeout: "3m"
          command:
            command: ["./prepare-canary.sh"]
            env:
              DEPLOYMENT_TYPE: "canary"
              PERCENTAGE: "10"
        - name: "stakeholder-notification"
          type: "http"
          failurePolicy: "ignore"
          http:
            url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
            method: "POST"
            headers:
              Content-Type: "application/json"
            body: |
              {
                "text": "🚀 Starting canary deployment (10%)",
                "channel": "#deployments"
              }
        postHooks:
        - name: "canary-validation"
          type: "command"
          failurePolicy: "fail"
          timeout: "15m"
          command:
            command: ["./validate-canary.sh"]
        - name: "update-dashboard"
          type: "http"
          failurePolicy: "ignore"
          http:
            url: "https://dashboard.example.com/api/deployment-status"
            method: "POST"
            headers:
              Authorization: "Bearer {{.secrets.dashboard.token}}"
              Content-Type: "application/json"
            body: |
              {
                "phase": "canary",
                "status": "completed",
                "percentage": 10
              }
              
      - name: "full-deployment"
        percentage: 90
        preHooks:
        - name: "production-readiness"
          type: "command"
          failurePolicy: "abort"
          timeout: "10m"
          command:
            command: ["./production-readiness-check.sh"]
            env:
              STRICT_MODE: "true"
        postHooks:
        - name: "deployment-complete"
          type: "http"
          failurePolicy: "ignore"
          http:
            url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
            method: "POST"
            body: |
              {
                "text": "🎉 Production deployment completed successfully!",
                "channel": "#deployments"
              }
  template:
    metadata:
      name: "{{name}}"
    spec:
      project: "production"
      source:
        repoURL: https://github.com/example/apps
        targetRevision: HEAD
        path: "{{path}}"
      destination:
        server: "{{server}}"
        namespace: "{{namespace}}"
```

### Hooks vs Checks

| Feature | Pre/Post Hooks | Validation Checks |
|---------|---------------|-------------------|
| **When** | Before/after deployment | During phase execution |
| **Purpose** | Setup, teardown, notifications | Health validation |
| **Failure Impact** | Can block deployment start/end | Can prevent phase progression |
| **Use Cases** | Infrastructure prep, notifications | Health checks, integration tests |

## Validation Checks

### HTTP Checks

Perform HTTP requests to validate deployment health:

```yaml
checks:
- name: "api-health"
  type: "http"
  http:
    url: "https://api.example.com/health"
    method: "GET"  # Optional, defaults to GET
    headers:
      Authorization: "Bearer {{.Values.token}}"
    expectedStatus: 200  # Optional, defaults to 200
    insecureSkipVerify: false  # Optional, for self-signed certs
  timeout: "30s"  # Optional, defaults to 5m
```

### Command Checks

Execute shell commands for custom validation:

```yaml
checks:
- name: "custom-validation"
  type: "command"
  command:
    command: ["./validate-deployment.sh"]
    env:
      CLUSTER_NAME: "{{.Values.cluster}}"
      APP_NAME: "{{.Values.name}}"
  timeout: "10m"
```

### Check Environment Variables

The following environment variables are automatically available in command checks:

- `APPSET_NAME`: Name of the ApplicationSet
- `APPSET_NAMESPACE`: Namespace of the ApplicationSet

## Failure Handling

Configure how the system responds to check failures:

```yaml
phases:
- name: "production-phase"
  targets:
  - clusters: ["prod"]
  checks:
  - name: "critical-check"
    type: "http"
    http:
      url: "https://health.example.com"
  onFailure:
    action: "stop"  # Options: stop, continue, rollback
```

### Failure Actions

- **stop** (default): Stop the deployment and do not proceed to subsequent phases
- **continue**: Log the failure but continue with the deployment
- **rollback**: Mark the phase for rollback (adds rollback annotation)

## Complete Examples

### Multi-Environment Deployment

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: progressive-deployment
spec:
  generators:
  - clusters:
      selector:
        matchLabels:
          environment: "*"
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: "development"
        targets:
        - matchExpressions:
          - key: "environment"
            operator: "In"
            values: ["dev", "development"]
        checks:
        - name: "dev-smoke-test"
          type: "command"
          command:
            command: ["curl", "-f", "http://{{name}}.dev.example.com/health"]
        waitDuration: "2m"
        
      - name: "staging"
        targets:
        - values:
            environment: "staging"
        checks:
        - name: "staging-health"
          type: "http"
          http:
            url: "https://{{name}}.staging.example.com/api/health"
            headers:
              X-Environment: "staging"
        - name: "integration-test"
          type: "command"
          command:
            command: ["npm", "run", "test:integration"]
            env:
              BASE_URL: "https://{{name}}.staging.example.com"
        onFailure:
          action: "stop"
        waitDuration: "5m"
        
      - name: "production-canary"
        targets:
        - matchExpressions:
          - key: "environment"
            operator: "In"
            values: ["prod", "production"]
          - key: "role"
            operator: "In"
            values: ["canary"]
        maxUpdate: 1
        checks:
        - name: "canary-health"
          type: "http"
          http:
            url: "https://{{name}}.prod.example.com/health"
            expectedStatus: 200
          timeout: "30s"
        onFailure:
          action: "rollback"
        waitDuration: "10m"
        
      - name: "production-full"
        targets:
        - matchExpressions:
          - key: "environment"
            operator: "In"
            values: ["prod", "production"]
        checks:
        - name: "full-deployment-health"
          type: "http"
          http:
            url: "https://monitoring.example.com/check-all-instances"
        onFailure:
          action: "stop"
  template:
    metadata:
      name: "{{name}}"
    spec:
      project: "default"
      source:
        repoURL: https://github.com/example/apps
        targetRevision: HEAD
        path: "{{path}}"
      destination:
        server: "{{server}}"
        namespace: "{{namespace}}"
```

### Percentage-based Canary Deployment

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: percentage-canary-deployment
spec:
  generators:
  - clusters:
      selector:
        matchLabels:
          environment: "production"
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: "canary-5-percent"
        percentage: 5  # Deploy 5% of applications as canary
        checks:
        - name: "canary-health"
          type: "http"
          http:
            url: "https://{{name}}.prod.example.com/health"
            headers:
              X-Canary-Check: "true"
        - name: "error-rate-check"
          type: "command"
          command:
            command: ["./check-error-rate.sh"]
            env:
              SERVICE: "{{name}}"
              THRESHOLD: "0.1%"
          timeout: "5m"
        waitDuration: "30m"  # Long observation period for canary
        onFailure:
          action: "stop"
          
      - name: "partial-rollout-25-percent"
        percentage: 25  # Deploy 25% more (30% total)
        checks:
        - name: "partial-health"
          type: "http"
          http:
            url: "https://{{name}}.prod.example.com/actuator/health"
        waitDuration: "15m"
        onFailure:
          action: "rollback"
          
      - name: "full-deployment-70-percent"
        percentage: 70  # Deploy remaining 70% (100% total)
        checks:
        - name: "full-health"
          type: "http"
          http:
            url: "https://{{name}}.prod.example.com/health"
        onFailure:
          action: "stop"
          
  template:
    metadata:
      name: "{{name}}"
    spec:
      project: "production"
      source:
        repoURL: https://github.com/example/manifests
        targetRevision: HEAD
        path: overlays/production
      destination:
        server: "{{server}}"
        namespace: "{{name}}"
```

### Git Generator with Regional Deployment

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: regional-rollout
spec:
  generators:
  - git:
      repoURL: https://github.com/example/apps
      revision: HEAD
      directories:
      - path: apps/*
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: "us-west"
        targets:
        - values:
            region: "us-west-2"
        checks:
        - name: "region-health"
          type: "http"
          http:
            url: "https://health-us-west.example.com/status"
            
      - name: "us-east"
        targets:
        - values:
            region: "us-east-1"
        checks:
        - name: "cross-region-sync"
          type: "command"
          command:
            command: ["./check-data-sync.sh"]
            env:
              SOURCE_REGION: "us-west-2"
              TARGET_REGION: "us-east-1"
  template:
    metadata:
      name: "{{path.basename}}"
    spec:
      project: "default"
      source:
        repoURL: https://github.com/example/apps
        targetRevision: HEAD
        path: "{{path}}"
      destination:
        server: "{{server}}"
        namespace: "{{path.basename}}"
```

### Combined Percentage and Cluster Filtering Example

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: enterprise-percentage-rollout
spec:
  generators:
  - clusters:
      selector:
        matchLabels:
          managed: "true"
    deploymentStrategy:
      type: phaseDeployment
      phases:
      # Phase 1: Deploy 15% to dev clusters only
      - name: "dev-canary-15-percent"
        percentage: 15
        targets:
        - matchExpressions:
          - key: "environment"
            operator: "In"
            values: ["dev", "development"]
        checks:
        - name: "dev-health"
          type: "http"
          http:
            url: "https://{{name}}.dev.example.com/health"
        waitDuration: "10m"
        
      # Phase 2: Deploy 25% to staging clusters (40% total)
      - name: "staging-25-percent"
        percentage: 25
        targets:
        - matchExpressions:
          - key: "environment"
            operator: "In"
            values: ["staging"]
        maxUpdate: 3  # Limit blast radius
        checks:
        - name: "staging-integration"
          type: "command"
          command:
            command: ["npm", "run", "test:integration"]
        waitDuration: "15m"
        
      # Phase 3: Deploy 30% to prod canary (70% total)
      - name: "prod-canary-30-percent"
        percentage: 30
        targets:
        - matchExpressions:
          - key: "environment"
            operator: "In"
            values: ["production"]
          - key: "tier"
            operator: "In"
            values: ["canary"]
        checks:
        - name: "prod-canary-validation"
          type: "http"
          http:
            url: "https://{{name}}.canary.prod.example.com/health"
        waitDuration: "30m"
        onFailure:
          action: "stop"
          
      # Phase 4: Deploy remaining 30% to all prod (100% total)
      - name: "prod-full-30-percent"
        percentage: 30
        targets:
        - matchExpressions:
          - key: "environment"
            operator: "In"
            values: ["production"]
        checks:
        - name: "full-prod-health"
          type: "http"
          http:
            url: "https://{{name}}.prod.example.com/health"
            
  template:
    metadata:
      name: "{{name}}"
    spec:
      project: "enterprise"
      source:
        repoURL: https://github.com/company/apps
        targetRevision: HEAD
        path: "apps/{{name}}"
      destination:
        server: "{{server}}"
        namespace: "{{name}}"
```

This example demonstrates:
- **Progressive environment rollout**: dev → staging → prod-canary → prod-full
- **Percentage control**: 15% → 25% → 30% → 30% (cumulative)
- **Cluster filtering**: Each phase targets specific cluster types
- **Risk management**: maxUpdate limits and comprehensive health checks

## Monitoring and Status

### ApplicationSet Annotations

The phase deployment processor automatically adds annotations to track progress:

```yaml
metadata:
  annotations:
    applicationset.argoproj.io/phase-list: "1"  # Current phase index
    applicationset.argoproj.io/rollback-phase-production: "2023-01-01T12:00:00Z"  # Rollback timestamp
```

### Phase Status API

You can query the current phase status programmatically:

```go
currentPhase, totalPhases, err := processor.GetGeneratorPhaseStatus(appSet, generator)
status := GeneratorPhaseStatusToJSON(currentPhase, totalPhases)
// Returns: {"currentPhase": 1, "totalPhases": 3, "completed": false}
```

## Best Practices

### 1. Start Small
Begin with simple two-phase deployments (dev → prod) before implementing complex multi-phase strategies.

### 2. Use Hooks for Non-Validation Tasks
- **Pre-hooks**: Infrastructure preparation, notifications, security scans
- **Post-hooks**: Cleanup, success notifications, metric collection
- **Checks**: Health validation, integration testing

### 3. Comprehensive Health Checks
Implement both technical health checks (HTTP endpoints) and business logic validation (custom commands).

### 4. Timeout Configuration
Always configure appropriate timeouts for hooks and checks to avoid indefinite waiting.

### 5. Failure Strategy
Choose failure strategies based on your risk tolerance:
- Use `fail` for critical pre-deployment validations
- Use `ignore` for optional notifications and cleanup
- Use `abort` for security failures that should stop everything
- Use `stop` for critical production deployments
- Use `continue` for development environments
- Use `rollback` when you have automated recovery procedures

### 6. Monitoring Integration
Integrate with your monitoring system by calling monitoring APIs in your checks and post-hooks.

### 7. Gradual Rollout
Use `maxUpdate` to control blast radius, especially in production environments.

### 8. Hook Design Patterns
- Keep hooks lightweight and focused on single responsibilities
- Use idempotent operations in hooks for reliability
- Implement proper error handling in custom hook scripts
- Use `failurePolicy: "ignore"` for non-critical operations

## Troubleshooting

### Common Issues

1. **Phase Not Advancing**: Check that validation checks are passing and wait durations have elapsed.

2. **Check Failures**: Review ApplicationSet controller logs for detailed error messages from failed checks.

3. **Timeout Issues**: Increase timeout values for slow-responding health endpoints or long-running validation commands.

4. **Target Matching**: Verify that your target expressions correctly match the generated parameters.

### Debugging

Enable debug logging to see detailed phase execution:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-applicationset-controller-config
data:
  log.level: "debug"
```

## Limitations

- Phase deployment is only supported at the generator level, not at the ApplicationSet level
- Nested generators (Matrix/Merge) inherit deployment strategy from their parent generators
- Hook and check commands are executed in the ApplicationSet controller pod environment
- HTTP hooks and checks do not support client certificates or advanced authentication methods
- Hook environment variables are limited to basic ApplicationSet and hook metadata