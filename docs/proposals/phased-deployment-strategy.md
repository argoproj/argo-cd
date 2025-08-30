---
title: ApplicationSet Phased Deployment Strategy with Pre/Post Hooks
authors:
  - "@gpamit"
sponsors:
  - TBD
reviewers:
  - "@alexmt"
  - "@crenshaw-dev"
  - "@jannfis"
approvers:
  - "@alexmt"
  - "@crenshaw-dev"
  - "@jannfis"

creation-date: 2025-01-17
last-updated: 2025-01-17
---

# ApplicationSet Phased Deployment Strategy with Pre/Post Hooks

Current Status: Alpha (Since v2.12.0)

This proposal introduces a sophisticated phased deployment strategy for ApplicationSet generators that enables progressive rollouts across multiple clusters with configurable pre/post hooks, health checks, and failure handling mechanisms.

## Summary

This enhancement adds a new `deploymentStrategy` field to ApplicationSet generators that supports phased deployments. The strategy allows users to define multiple deployment phases, each targeting specific clusters or environments, with configurable pre and post hooks for validation, notifications, and custom actions. This enables safe, progressive rollouts with automated quality gates and comprehensive deployment orchestration.

The implementation includes:
- **Phased Deployment Strategy**: Deploy applications progressively across multiple phases
- **Pre/Post Hooks**: Execute HTTP calls or commands before and after each phase
- **Health Checks**: Validate application health between phases
- **Failure Handling**: Configurable actions on failure (stop, rollback, abort)
- **Target Selection**: Flexible cluster targeting using selectors and expressions
- **Progressive Rollouts**: Percentage-based and count-based deployment controls

## Motivation

Current ApplicationSet implementations deploy to all clusters simultaneously, which poses significant risks in production environments. Organizations need:

1. **Progressive Rollouts**: Deploy to dev/staging before production
2. **Automated Quality Gates**: Run validation and health checks between phases
3. **Integration Capabilities**: Trigger external systems (notifications, ITSM, monitoring)
4. **Failure Safety**: Automatic rollback or deployment halting on failures
5. **Compliance**: Audit trails and change management integration
6. **Observability**: Rich deployment status tracking and reporting

### Goals

- Enable safe, progressive application deployments across multiple clusters
- Provide configurable hooks for external system integration
- Implement comprehensive failure handling and rollback mechanisms
- Support both percentage-based and count-based phased rollouts
- Maintain backward compatibility with existing ApplicationSet functionality
- Deliver enterprise-grade deployment orchestration capabilities

### Non-Goals

- Replace Argo Rollouts for application-level progressive delivery
- Implement complex canary analysis or traffic splitting
- Provide built-in approval workflows (delegated to external systems via hooks)
- Support cross-ApplicationSet deployment dependencies

## Proposal

### Use Cases

#### Use case 1: Progressive Environment Rollout
As a platform engineer, I want to deploy applications first to development clusters, then staging, and finally production, with health checks between each phase to ensure deployment quality.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: progressive-rollout
spec:
  generators:
  - clusters: {}
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: development
        targets:
        - matchExpressions:
          - key: environment
            operator: In
            values: ["dev"]
        checks:
        - name: health-check
          type: http
          http:
            url: "https://{{name}}.dev.example.com/health"
        waitDuration: "2m"
      - name: staging
        targets:
        - matchExpressions:
          - key: environment
            operator: In
            values: ["staging"]
        checks:
        - name: health-check
          type: http
          http:
            url: "https://{{name}}.staging.example.com/health"
        waitDuration: "5m"
      - name: production
        targets:
        - matchExpressions:
          - key: environment
            operator: In
            values: ["prod"]
        onFailure:
          action: rollback
```

#### Use case 2: Canary Deployment with External Integration
As a DevOps engineer, I want to deploy to 10% of production clusters first, validate with external monitoring systems, send notifications, and then proceed to full deployment.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: canary-with-hooks
spec:
  generators:
  - clusters: {}
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: canary
        percentage: 10
        targets:
        - matchExpressions:
          - key: environment
            operator: In
            values: ["production"]
        preHooks:
        - name: deployment-notification
          type: http
          http:
            url: "https://hooks.slack.com/webhook"
            method: POST
            body: '{"text": "Starting canary deployment"}'
        postHooks:
        - name: monitoring-validation
          type: command
          command:
            command: ["./scripts/validate-metrics.sh"]
        waitDuration: "10m"
      - name: full-production
        percentage: 100
        targets:
        - matchExpressions:
          - key: environment
            operator: In
            values: ["production"]
```

#### Use case 3: Compliance and Change Management Integration
As a compliance officer, I want all production deployments to create change requests in ServiceNow, wait for approval, and maintain audit trails.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: compliance-deployment
spec:
  generators:
  - clusters: {}
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: production
        targets:
        - matchExpressions:
          - key: environment
            operator: In
            values: ["production"]
        preHooks:
        - name: create-change-request
          type: http
          failurePolicy: fail
          http:
            url: "https://api.servicenow.com/api/now/table/change_request"
            method: POST
            headers:
              Authorization: "Bearer {{.secrets.servicenow.token}}"
            body: |
              {
                "short_description": "Production deployment for {{.appset.name}}",
                "category": "Software"
              }
        - name: wait-for-approval
          type: command
          timeout: "24h"
          command:
            command: ["./scripts/wait-for-approval.sh"]
```

### Implementation Details

#### Core Components

1. **Complete DeploymentStrategy Structure**
```yaml
deploymentStrategy:
  type: phaseDeployment  # Required: strategy type
  phases:               # Required: list of deployment phases
  - name: string        # Required: phase identifier
    percentage: int64   # Optional: percentage of targets (0-100, mutually exclusive with maxUpdate)
    maxUpdate: intOrString  # Optional: max targets to update (mutually exclusive with percentage)
    targets: []         # Required: target selection criteria
    preHooks: []        # Optional: hooks to execute before phase
    postHooks: []       # Optional: hooks to execute after phase
    checks: []          # Optional: health checks during phase
    waitDuration: string # Optional: wait time after phase completion (e.g., "5m", "30s")
    onFailure:          # Optional: failure handling configuration
      action: string    # "stop", "rollback", "continue"
```

2. **Target Selection Structure**
```yaml
targets:
- clusters: ["cluster1", "cluster2"]  # Explicit cluster names
- values:                             # Key-value matching
    environment: "production"
    region: "us-west-2"
- matchExpressions:                   # Kubernetes-style selectors
  - key: "environment"
    operator: "In"                    # "In", "NotIn", "Exists", "DoesNotExist"
    values: ["prod", "staging"]
```

3. **Hook Types with Full Configuration**

**HTTP Hooks**:
```yaml
- name: webhook-notification
  type: http
  timeout: "30s"                    # Optional: default 5m
  failurePolicy: "fail"             # "fail", "ignore", "abort"
  http:
    url: "https://webhook.example.com"
    method: "POST"                  # Optional: default "POST" for hooks, "GET" for checks
    headers:                        # Optional: custom headers
      Content-Type: "application/json"
      Authorization: "Bearer token"
    body: |                         # Optional: request body
      {"message": "Deployment started"}
    expectedStatus: 200             # Optional: default 200
    insecureSkipVerify: false       # Optional: skip TLS verification
```

**Command Hooks**:
```yaml
- name: validation-script
  type: command
  timeout: "10m"                    # Optional: default 5m
  failurePolicy: "fail"             # "fail", "ignore", "abort"
  command:
    command: ["./scripts/validate.sh", "--env", "prod"]
    env:                            # Optional: custom environment variables
      DEPLOYMENT_ENV: "production"
      API_ENDPOINT: "https://api.example.com"
```

4. **Health Checks with Full Configuration**

**HTTP Checks**:
```yaml
checks:
- name: application-health
  type: http
  timeout: "2m"                     # Optional: default 5m
  http:
    url: "https://{{name}}.example.com/health"
    method: "GET"                   # Optional: default "GET" for checks
    headers:                        # Optional: custom headers
      User-Agent: "ArgoCD-HealthCheck"
    expectedStatus: 200             # Optional: default 200
    insecureSkipVerify: false       # Optional: skip TLS verification
```

**Command Checks**:
```yaml
checks:
- name: cluster-readiness
  type: command
  timeout: "5m"                     # Optional: default 5m
  command:
    command: ["kubectl", "get", "nodes", "--no-headers"]
    env:                            # Optional: custom environment variables
      KUBECONFIG: "/etc/kubeconfig"
```

5. **Automatic Environment Variables**

The implementation automatically injects environment variables for all hooks and checks:
- `APPSET_NAME`: ApplicationSet name
- `APPSET_NAMESPACE`: ApplicationSet namespace
- `HOOK_NAME`: Hook name (for hooks only)
- `HOOK_TYPE`: Hook type ("pre" or "post", for hooks only)

HTTP requests also get automatic headers:
- `X-AppSet-Name`: ApplicationSet name
- `X-AppSet-Namespace`: ApplicationSet namespace
- `X-Hook-Name`: Hook name
- `X-Hook-Type`: Hook type
- `User-Agent`: "ArgoCD-ApplicationSet-PhaseHook/1.0"

#### Phase Execution Flow

1. **Phase Initialization**: Select target clusters based on criteria and sort for consistent ordering
2. **Pre-Hook Execution**: Run all pre-hooks with configurable concurrency (default: max 5 concurrent)
3. **Application Deployment**: Deploy/update applications to selected clusters
4. **Health Validation**: Execute health checks with configurable concurrency (default: max 10 concurrent)
5. **Post-Hook Execution**: Run all post-hooks with configurable concurrency (default: max 5 concurrent)
6. **Wait Period**: Context-aware sleep with cancellation support
7. **Phase Advancement**: Check criteria and advance to next phase or complete
8. **Failure Handling**: Execute failure action with detailed logging and rollback annotations

#### Concurrency and Performance

The implementation includes sophisticated performance optimizations:

- **Concurrent Execution**: Hooks and checks run concurrently within phases
- **Configurable Limits**:
  - Max 5 concurrent hooks per phase
  - Max 10 concurrent health checks per phase
  - Max 1000 applications per phase
- **Resource Limits**:
  - Max 1MB HTTP response body size
  - Max 30-minute command timeout
  - Max 10-minute HTTP timeout
  - Max 100 environment variables per command
  - Max 50 HTTP headers per request

#### Phase State Management

Phase progression is tracked using Kubernetes annotations:
- `applicationset.argoproj.io/phase-{generator-type}`: Current phase index
- `applicationset.argoproj.io/rollback-phase-{phase-name}`: Rollback timestamps

Phase advancement logic:
- **Target-based phases**: Advance when no matching targets remain
- **Percentage-based phases**: Advance when current percentage allocation is complete
- **Mixed strategies**: Support both percentage and target-based phases in same deployment

#### Status Tracking and Monitoring

Enhanced ApplicationSet status includes:
- Current phase information with detailed progress
- Per-application deployment status with step tracking
- Hook execution results with timestamps and duration
- Failure details and remediation actions
- JSON-formatted phase status for external monitoring

Status API includes:
```go
func GetGeneratorPhaseStatus(appSet, generator) (currentPhase, totalPhases int, error)
func GeneratorPhaseStatusToJSON(currentPhase, totalPhases int) string
```

### Security Considerations

The implementation includes comprehensive security measures:

#### Command Security
- **Command Validation**: Prevents dangerous command patterns and validates input
- **Path Traversal Protection**: Validates command paths and prevents absolute path attacks
- **Environment Variable Limits**: Max 100 environment variables with suspicious pattern detection
- **Blacklist Scanning**: Detects potentially dangerous commands (rm, sudo, curl, etc.)
- **Execution Isolation**: Commands run in controlled contexts with timeout enforcement

#### HTTP Security
- **URL Validation**: Validates URL schemes and prevents internal network access
- **TLS Enforcement**: Encourages HTTPS usage with configurable certificate verification
- **Header Validation**: Limits to 50 headers with sensitive header detection
- **Response Size Limits**: Max 1MB response body to prevent memory exhaustion
- **Internal Network Protection**: Warns about localhost/private IP access

#### Access Control
- **Credential Management**: Hooks access secrets through standard Kubernetes mechanisms
- **RBAC Integration**: Hook execution respects existing RBAC policies
- **Network Security**: HTTP hooks use TLS and certificate validation by default
- **Audit Logging**: All hook executions are logged with detailed context

#### Version Compatibility
- **Minimum Version Validation**: Checks for `argocd.argoproj.io/min-version` annotation
- **Semantic Version Support**: Validates version format and compatibility
- **Backward Compatibility**: Supports deployments without version requirements

### Risks and Mitigations

**Risk**: Hook failures could block deployments indefinitely
**Mitigation**:
- Configurable timeouts (default 5m, max 30m for commands, max 10m for HTTP)
- Failure policies: "fail" (default), "ignore", "abort"
- Context-aware cancellation for graceful shutdowns
- Performance warnings for large hook counts (>10 hooks)

**Risk**: Security vulnerabilities in command execution
**Mitigation**:
- Comprehensive command security validation
- Blacklist scanning for dangerous commands
- Environment variable limits and validation
- Path traversal protection
- Audit logging of all executions

**Risk**: Performance impact on ApplicationSet controller
**Mitigation**:
- Concurrent execution with configurable limits
- Resource usage warnings and limits
- Efficient phase state management
- Performance monitoring and optimization

**Risk**: External system dependencies for hooks
**Mitigation**:
- Retry mechanisms and graceful degradation options
- Timeout enforcement with context cancellation
- Detailed error logging and status reporting
- Failure policy options for different scenarios

**Risk**: Potential for deployment inconsistencies across phases
**Mitigation**:
- Rollback mechanisms with annotation tracking
- Comprehensive status tracking and reporting
- Deterministic parameter ordering and filtering
- Phase advancement validation

**Risk**: Memory and resource exhaustion
**Mitigation**:
- Response body size limits (1MB)
- Application count limits per phase (1000)
- Concurrent execution limits (5 hooks, 10 checks)
- Environment variable limits (100 per command)
- HTTP header limits (50 per request)

### Upgrade / Downgrade Strategy

**Upgrade**:
- New `deploymentStrategy` field is optional and backward-compatible
- Existing ApplicationSets continue working without changes
- Users can gradually adopt phased deployments per ApplicationSet

**Downgrade**:
- Remove `deploymentStrategy` field to revert to original behavior
- In-progress phased deployments complete current phase before reverting
- No data loss or corruption during downgrade

## Detailed Examples

### Basic Progressive Rollout
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: basic-progressive
spec:
  generators:
  - clusters: {}
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: development
        targets:
        - clusters: ["dev-us-west", "dev-eu-west"]
        waitDuration: "5m"
      - name: staging
        targets:
        - clusters: ["staging-us-west", "staging-eu-west"]
        waitDuration: "10m"
      - name: production
        targets:
        - clusters: ["prod-us-west", "prod-eu-west"]
        onFailure:
          action: stop
  template:
    metadata:
      name: "{{name}}"
    spec:
      project: default
      source:
        repoURL: https://github.com/example/app
        targetRevision: HEAD
        path: manifests
      destination:
        server: "{{server}}"
        namespace: default
```

### Advanced Hook Integration
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: advanced-hooks
spec:
  generators:
  - clusters: {}
    deploymentStrategy:
      type: phaseDeployment
      phases:
      - name: production
        targets:
        - matchExpressions:
          - key: tier
            operator: In
            values: ["production"]
        preHooks:
        - name: infrastructure-check
          type: command
          timeout: "5m"
          failurePolicy: fail
          command:
            command: ["kubectl", "get", "nodes", "--no-headers"]
            env:
              KUBECONFIG: "/etc/kubeconfig"
        - name: slack-notification
          type: http
          timeout: "30s"
          failurePolicy: ignore
          http:
            url: "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"
            method: POST
            headers:
              Content-Type: "application/json"
            body: |
              {
                "text": "🚀 Starting production deployment for {{ .appset.name }}",
                "channel": "#deployments"
              }
        postHooks:
        - name: smoke-tests
          type: command
          timeout: "15m"
          failurePolicy: fail
          command:
            command: ["pytest", "tests/smoke/"]
            env:
              TARGET_URL: "https://{{name}}.example.com"
        - name: success-notification
          type: http
          failurePolicy: ignore
          http:
            url: "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"
            method: POST
            headers:
              Content-Type: "application/json"
            body: |
              {
                "text": "✅ Production deployment completed for {{ .appset.name }}",
                "channel": "#deployments"
              }
  template:
    metadata:
      name: "{{name}}"
    spec:
      project: default
      source:
        repoURL: https://github.com/example/app
        targetRevision: HEAD
        path: k8s
      destination:
        server: "{{server}}"
        namespace: "{{name}}"
```

## Drawbacks

1. **Increased Complexity**: Adds significant complexity to ApplicationSet controller
2. **External Dependencies**: Hooks create dependencies on external systems
3. **Longer Deployment Times**: Phased approach increases total deployment duration
4. **Resource Usage**: Additional controller resources for phase management and hook execution
5. **Debugging Challenges**: More complex failure scenarios and troubleshooting

## Alternatives

### Alternative 1: Separate Deployment Orchestrator
Create a separate controller specifically for deployment orchestration that manages multiple ApplicationSets.

**Pros**: Clean separation of concerns, no ApplicationSet complexity
**Cons**: Additional component to maintain, more complex architecture

### Alternative 2: GitOps-Based Progression
Use Git commits/branches to trigger progressive deployments through existing CI/CD pipelines.

**Pros**: Leverages existing GitOps patterns, simple implementation
**Cons**: Less flexible, requires external orchestration, no built-in failure handling

### Alternative 3: Argo Workflows Integration
Use Argo Workflows to orchestrate ApplicationSet deployments across phases.

**Pros**: Leverages existing Argo ecosystem, powerful workflow capabilities
**Cons**: Requires additional Argo component, complex setup for simple use cases

### Alternative 4: External Progressive Delivery Tools
Use tools like Flagger or Argo Rollouts at the ApplicationSet level.

**Pros**: Mature progressive delivery features
**Cons**: Not designed for multi-cluster ApplicationSet scenarios, architectural mismatch

## Integration with Existing ApplicationSet Features

### Relationship to ApplicationSet Rolling Strategy

This phased deployment strategy complements the existing ApplicationSet rolling strategy:

- **ApplicationSet Level**: The existing `ApplicationSetStrategy` with `RollingSync` provides step-based progressive updates across all generators
- **Generator Level**: The new `GeneratorDeploymentStrategy` with `phaseDeployment` provides fine-grained control per generator with hooks and checks

The two strategies can coexist:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  strategy:                    # ApplicationSet-level strategy
    type: RollingSync
    rollingSync:
      steps:
      - matchExpressions:
        - key: env
          operator: In
          values: ["dev"]
  generators:
  - clusters: {}
    deploymentStrategy:        # Generator-level strategy
      type: phaseDeployment
      phases: [...]
```

### Controller Integration

The phased deployment functionality integrates seamlessly with the existing ApplicationSet controller:

1. **Generator Processing**: Hooks into the existing generator parameter processing pipeline
2. **Application Management**: Uses existing Application resource creation and update mechanisms
3. **Status Reporting**: Extends existing ApplicationSet status tracking
4. **Error Handling**: Integrates with existing ApplicationSet error reporting and conditions

### Performance Considerations

The implementation is designed to minimize impact on the ApplicationSet controller:

- **Conditional Processing**: Only activates when `deploymentStrategy.type: phaseDeployment` is specified
- **Non-blocking Operations**: Uses goroutines and channels for concurrent execution
- **Memory Management**: Implements strict limits to prevent resource exhaustion
- **Efficient State Management**: Uses annotations for minimal storage overhead

## Testing Strategy

### Unit Testing

Comprehensive unit test coverage includes:

- **Phase filtering logic**: Target selection, percentage calculations, maxUpdate constraints
- **Hook execution**: HTTP and command hook validation and execution
- **Security validation**: Command and HTTP security checks
- **Error handling**: Failure scenarios and recovery mechanisms
- **Concurrency**: Concurrent hook and check execution

### Integration Testing

Integration tests validate:

- **End-to-end phase progression**: Complete phase deployment cycles
- **External system integration**: Real HTTP webhook and command execution
- **ApplicationSet controller integration**: Full controller workflow
- **Status tracking**: ApplicationSet status updates and condition management

### Performance Testing

Performance tests ensure:

- **Scalability**: Testing with large numbers of clusters and hooks
- **Resource limits**: Validation of memory and CPU constraints
- **Concurrent execution**: Stress testing concurrent operations
- **Timeout handling**: Proper timeout enforcement and cleanup

## Documentation Requirements

### User Documentation

Required documentation additions:

1. **User Guide**: Complete guide for configuring phased deployments
2. **Examples Repository**: Real-world examples and best practices
3. **Troubleshooting Guide**: Common issues and debugging techniques
4. **Security Guide**: Security considerations and best practices

### API Documentation

API documentation updates:

1. **CRD Schema**: Complete OpenAPI schema with validation rules
2. **Field References**: Detailed field documentation with examples
3. **Hook Reference**: Complete hook type and configuration reference
4. **Status API**: Documentation of status tracking and monitoring APIs

## Implementation Phases

### Phase 1: Core Implementation ✅
- Basic phase deployment functionality
- HTTP and command hooks
- Target selection and filtering
- Security validation

### Phase 2: Advanced Features ✅
- Concurrent execution optimization
- Comprehensive status tracking
- Rollback mechanism implementation
- Performance monitoring

### Phase 3: Enterprise Features ✅
- Advanced security controls
- Audit logging integration
- Version compatibility checking
- Resource limit enforcement

### Phase 4: Documentation and Testing
- Complete user documentation
- Integration test coverage
- Performance benchmarking
- Security audit

## Conclusion

The proposed phased deployment strategy significantly enhances ArgoCD's multi-cluster deployment capabilities, providing enterprise-grade deployment orchestration with comprehensive hooks, health checks, and failure handling. While it adds complexity, the benefits of safe, progressive deployments with external system integration far outweigh the drawbacks for organizations managing large-scale, multi-cluster environments.

This enhancement positions ArgoCD as a complete solution for sophisticated deployment scenarios while maintaining its core GitOps principles and ease of use for simpler deployments.

The implementation demonstrates careful consideration of security, performance, and integration concerns, making it suitable for production environments with enterprise requirements. The modular design ensures backward compatibility while providing powerful new capabilities for progressive deployment orchestration.
