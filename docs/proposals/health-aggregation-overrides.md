---
title: Health Aggregation Overrides
authors:
  - '@agaudreault'
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-01-15
last-updated: 2026-03-10
---

# Health Aggregation Overrides

Introduce configurable health aggregation behavior to allow users to customize how resource health statuses are aggregated into Application health, addressing common use cases where default aggregation behavior doesn't match operational intent.

## Open Questions

~~Is there a way to provide sensible defaults that could be defined in the `resource_customizations/` folder instead of on the argocd-cm?~~

**Resolved**: Alternative 2 (Lua-based approach) solves this by allowing defaults to be shipped in `resource_customizations/` folder via Lua health check scripts.

## Summary

Currently, Argo CD aggregates resource health into Application health using a "worst health wins" algorithm. This causes issues when resources are intentionally in states like "Suspended" (e.g., suspended Jobs/CronJobs for DR scenarios) or when certain resource types shouldn't affect Application health (e.g., ConfigMaps). This proposal introduces annotation-based and ConfigMap-based configuration to customize health aggregation behavior per resource or per Kind.

**User Impact**: Users will be able to prevent suspended Jobs/CronJobs from marking their Applications as suspended, ignore certain resource types from health aggregation, and map resource health statuses to different values for aggregation purposes.

## Requirements

This proposal aims to satisfy the following key requirements:

### R1: Distinct Health States for Resource and Aggregation

**Requirement**: The resource tree must show the individual resource's actual health state (e.g., "Suspended", "Degraded") with the correct icon and status, while allowing that resource to contribute a different health state (e.g., "Healthy") to the Application's aggregated health.

**Rationale**: Users need visibility into the actual state of individual resources for monitoring and debugging purposes. However, certain states (like "Suspended" for a CronJob during maintenance) should not cause the entire Application to be marked as unhealthy. This is a critical distinction that cannot be achieved by simply remapping the resource's health status.

**Examples**:

- A suspended CronJob should display as "Suspended" in the resource tree (so operators know it's suspended), but contribute "Healthy" to the Application's aggregated health (so the Application remains healthy)
- A Deployment with zero replicas might show as "Suspended" in the resource tree, but contribute "Healthy" to aggregation if this is an intentional scale-down

### R2: Default Behavior for Common Cases

**Requirement**: Argo CD should ship with sensible defaults for common resource types (Jobs, CronJobs) so that users don't need to configure every resource individually.

**Rationale**: The current behavior where suspended Jobs/CronJobs mark Applications as "Suspended" is a common pain point reported in multiple issues (#19126, #24428, #25551). Providing good defaults improves the out-of-box experience.

**Examples**:

- Suspended Jobs should default to contributing "Healthy" to aggregation
- Suspended CronJobs should default to contributing "Healthy" to aggregation

### R3: Per-Resource Override Capability

**Requirement**: Users must be able to override the default or Kind-level behavior for specific resource instances using annotations.

**Rationale**: While defaults work for most cases, some resources may need different behavior. For example, a critical Job that should affect Application health even when suspended.

**Examples**:

- Most Jobs use the default (Suspended → Healthy), but one critical Job is annotated to use original behavior (Suspended → Suspended)

### R4: Kind-Level Configuration

**Requirement**: Users must be able to configure health aggregation behavior for all resources of a specific Kind without annotating each resource individually.

**Rationale**: When managing many resources of the same type, per-resource annotations become impractical. Kind-level configuration provides a scalable solution.

**Examples**:

- Configure all Jobs in the cluster to treat Suspended as Healthy
- Configure all custom resources of a specific Kind to ignore certain health states

### R5: Backward Compatibility

**Requirement**: Existing Applications must continue to work without changes. The feature should be opt-in or have carefully considered defaults.

**Rationale**: Breaking existing Applications would disrupt users. Any default behavior changes must be well-justified and documented.

**Note**: While R2 suggests shipping with defaults for Jobs/CronJobs, this represents a behavior change that needs community consensus. The implementation must support both enabling and disabling these defaults.

## Motivation

### Problems with Current Behavior

1. **Suspended Jobs/CronJobs** ([#19126](https://github.com/argoproj/argo-cd/issues/19126)): When Jobs or CronJobs have `spec.suspend: true` (introduced in K8s v1.24), the Application becomes "Suspended" even though this is intentional and desired behavior. This affects:
   - DR/emergency jobs deployed alongside services
   - Jobs managed by external controllers
   - Monitoring alerts that trigger incorrectly
   - Scripts that check app health

2. **Resources Without Health Significance**: Some resources don't have meaningful health at the Application level, but currently affect Application health, requiring an annotation on each resource.

3. **Operational Intent Mismatch**: Zero-replica Deployments, suspended CronJobs, and similar intentional states are treated as unhealthy when they represent valid operational states.

**User Quotes from Issues**:

> "We have Jobs that are suspended by default and can be manually triggered. These Jobs mark our Application as Suspended, which triggers false alerts in our monitoring system." - Issue #19126

> "During maintenance windows, we suspend CronJobs. This causes all our Applications to show as Suspended, making it impossible to distinguish between Applications that are actually having issues and those that are just in maintenance." - Issue #24428

> "I want to see in the UI that my CronJob is suspended (so I know it's not running), but I don't want the Application health to be affected because the suspension is intentional." - Issue #19126

These quotes demonstrate that users explicitly need **both** visibility into resource state **and** correct Application health - exactly what R1 requires.

### Why Existing Mechanisms Are Insufficient

| Mechanism                         | Limitation                                          | Missing Capability                                                      |
| --------------------------------- | --------------------------------------------------- | ----------------------------------------------------------------------- |
| **Custom Health Check**           | Remaps resource health status                       | Cannot maintain distinct display vs aggregation health (R1)             |
| **ignore-healthcheck annotation** | Completely removes resource from health calculation | Resource won't affect health even if it becomes Degraded; no visibility |
| **No configuration**              | Uses default "worst health wins"                    | Suspended Jobs/CronJobs incorrectly mark Application as Suspended       |

### Visual Example: The Key Difference

Consider an Application with a Deployment, a Service, and a suspended CronJob:

**Current Behavior (No Configuration)**:

```
Application: my-app
├─ Status: Suspended ❌ (INCORRECT - app is not suspended)
├─ Deployment: web
│  └─ Health: Healthy ✅
├─ Service: web-svc
│  └─ Health: Healthy ✅
└─ CronJob: backup
   └─ Health: Suspended ⏸️
```

**Alternative 3 (Custom Health Check Override)**:

```
Application: my-app
├─ Status: Healthy ✅ (correct)
├─ Deployment: web
│  └─ Health: Healthy ✅
├─ Service: web-svc
│  └─ Health: Healthy ✅
└─ CronJob: backup
   └─ Health: Healthy ❤️ (MISLEADING - operator can't see it's suspended)
```

**This Proposal (Alternative 1 or 2)**:

```
Application: my-app
├─ Status: Healthy ✅ (correct)
├─ Deployment: web
│  └─ Health: Healthy ✅
├─ Service: web-svc
│  └─ Health: Healthy ✅
└─ CronJob: backup
   └─ Health: Suspended ⏸️ (correct - operator can see actual state)
   └─ Aggregation: Healthy (used for Application health)
```

### Goals

1. Allow users to override how individual resource health statuses are aggregated into Application health
2. Support both per-resource (annotation) and per-Kind (ConfigMap) configuration
3. Maintain backward compatibility - existing behavior unchanged unless explicitly configured
4. Provide simple, easy-to-understand configuration syntax
5. Ship with sensible defaults for common cases (suspended Jobs/CronJobs)
6. Solve [#19126](https://github.com/argoproj/argo-cd/issues/19126) and related issues

### Non-Goals

1. Changing how individual resource health is calculated (that's already customizable via Lua)
2. Changing the core aggregation algorithm (`health.IsWorse()`)
3. Supporting aggregation based on resource relationships or dependencies
4. Modifying health status display in UI (only affects aggregation logic)

## Proposal

This proposal presents **three alternative approaches**:

1. **Alternative 1 (ConfigMap-based)**: Introduces a new ConfigMap key `resource.customizations.health-aggregation.<group>_<kind>` for simple status mappings
2. **Alternative 2 (Lua-based, recommended)**: Extends existing Lua health checks to return an optional `aggregationHealth` field
3. **Alternative 3 (Custom Health Check Override)**: Use existing custom health check mechanism with annotations to remap health statuses

Alternatives 1 and 2 support per-resource annotation overrides. **Alternative 2 is recommended** because it:

- Reuses existing health check mechanism (simpler architecture)
- Allows defaults to be shipped in `resource_customizations/` folder
- Provides more flexibility for conditional logic
- Keeps health calculation and aggregation configuration in one place
- **Satisfies R1**: Maintains distinct health states for resource display and aggregation

**Alternative 3 does not satisfy R1** (the critical requirement for distinct health states) and is included only for comparison purposes.

See detailed comparison in the implementation sections below.

### Use Cases

The following use cases demonstrate how this proposal satisfies the requirements, particularly **R1 (distinct health states)**.

#### Use Case 1: Suspended Jobs for DR Scenarios (Addresses R1, R2, R4)

**Scenario**: As an operator, I deploy a MySQL database with a suspended backup Job. The Job is part of my Helm chart and can be manually triggered during DR scenarios. I want my Application to show as "Healthy" even though the Job is suspended, because suspension is intentional.

**Current Problem**:

- Application health: Suspended ❌ (incorrect - Application is not actually suspended)
- Job display in resource tree: Suspended ⏸️ (correct)

**Desired Behavior (R1)**:

- Application health: Healthy ✅ (Job suspension doesn't affect Application)
- Job display in resource tree: Suspended ⏸️ (operators can see the Job is suspended)

**Solution**: Configure Kind-level mapping for Jobs to treat Suspended as Healthy for aggregation purposes.

**Why Alternative 3 Fails**: Custom health check would show Job as "Healthy" in resource tree, hiding the fact that it's suspended.

#### Use Case 2: Suspended CronJobs for Maintenance Windows (Addresses R1, R2, R4)

**Scenario**: As a developer, I have CronJobs that run periodic tasks. During maintenance windows, I suspend these CronJobs. I don't want my Application to show as "Suspended" during this time.

**Current Problem**:

- Application health: Suspended ❌ (incorrect - Application is operational)
- CronJob display in resource tree: Suspended ⏸️ (correct)

**Desired Behavior (R1)**:

- Application health: Healthy ✅ (CronJob suspension is intentional)
- CronJob display in resource tree: Suspended ⏸️ (operators can see which CronJobs are suspended)

**Solution**: Configure Kind-level mapping for CronJobs to treat Suspended as Healthy for aggregation purposes.

**Why Alternative 3 Fails**: Custom health check would show CronJob as "Healthy" in resource tree, making it impossible to see which CronJobs are currently suspended.

#### Use Case 3: Progressing HPA During Scale Operations (Addresses R1)

**Scenario**: As a platform engineer, I have HorizontalPodAutoscalers that show as "Progressing" during normal scale operations. I don't want my Application to show as "Progressing" every time the HPA adjusts replica counts, as this is normal behavior.

**Current Problem**:

- Application health: Progressing ⚠️ (misleading - this is normal operation)
- HPA display in resource tree: Progressing (correct - HPA is actively scaling)

**Desired Behavior (R1)**:

- Application health: Healthy ✅ (HPA scaling is normal operation)
- HPA display in resource tree: Progressing ⚠️ (operators can see HPA is actively scaling)

**Solution**: Configure Kind-level mapping for HPAs to treat Progressing as Healthy for aggregation purposes.

**Why Alternative 3 Fails**: Custom health check would show HPA as "Healthy" in resource tree, hiding the fact that it's actively scaling.

**Note**: This use case can be addressed with existing mechanisms (`argocd.argoproj.io/ignore-healthcheck: "true"`), but this proposal provides a more nuanced solution that preserves visibility while controlling aggregation.

#### Use Case 4: Per-Resource Override for Critical Jobs (Addresses R1, R3)

**Scenario**: As a developer, most of my Jobs should affect health normally, but one specific Job is a manual-trigger Job that should be treated as healthy when suspended.

**Current Problem**: Cannot have different behavior for different Jobs of the same Kind without custom health checks that hide the actual state.

**Desired Behavior (R1)**:

- Application health: Healthy ✅ (manual Job suspension doesn't affect Application)
- Manual Job display in resource tree: Suspended ⏸️ (operators can see it's suspended)
- Other Jobs: Use default behavior (Suspended → Suspended if that's the default)

**Solution**: Add annotation to that specific Job to override its health aggregation. The annotation will be merged with Kind-level configuration, with annotation values taking precedence.

**Why Alternative 3 Fails**: Would require complex Lua logic to read annotations and conditionally remap health, and would still hide the actual state in the resource tree.

#### Use Case 5: Critical Job That Must Affect Health (Addresses R3)

**Scenario**: As an operator, I have a default configuration where suspended Jobs don't affect Application health. However, one specific Job is critical and should affect Application health even when suspended.

**Current Problem**: With default configuration, all suspended Jobs are treated as healthy for aggregation.

**Desired Behavior (R1)**:

- Application health: Suspended ❌ (critical Job is suspended)
- Critical Job display in resource tree: Suspended ⏸️ (operators can see it's suspended)
- Other Jobs: Use default behavior (Suspended → Healthy)

**Solution**: Add annotation to the critical Job to override the default:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: critical-job
  annotations:
    # Override default: this Job should affect health even when suspended
    argocd.argoproj.io/health-aggregation: 'Suspended=Suspended'
spec:
  suspend: true
```

This demonstrates the **precedence model**: Annotation > Kind-level config > Default behavior.

### Implementation Details/Notes/Constraints

Two alternative approaches are proposed below. **Alternative 2 (Lua-based)** is recommended as it's simpler, more consistent with existing patterns, and solves the open question about providing defaults.

---

## Alternative 1: ConfigMap-Based Health Aggregation Mapping

This approach introduces a new ConfigMap key `resource.customizations.health-aggregation.<group>_<kind>` to define mappings.

#### Configuration Format

**Per-Resource Annotation** (highest precedence):

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: manual-backup
  annotations:
    # Single mapping
    argocd.argoproj.io/health-aggregation: 'Suspended=Healthy'
    # Or multiple mappings (comma-separated)
    # argocd.argoproj.io/health-aggregation: 'Suspended=Healthy,Progressing=Healthy'
spec:
  suspend: true
  # ... rest of spec
```

**Kind-Level Configuration in argocd-cm**:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  # Map suspended status to healthy for all Jobs
  resource.customizations.health-aggregation.batch_Job: |
    Suspended: Healthy

  # Map suspended status to healthy for all CronJobs
  resource.customizations.health-aggregation.batch_CronJob: |
    Suspended: Healthy

  # Wildcard example: Match all resources in custom.io group
  resource.customizations.health-aggregation.custom.io_*: |
    Suspended: Healthy
    Progressing: Healthy
```

**Configuration Precedence** (highest to lowest):

1. **Per-resource annotation** - `argocd.argoproj.io/health-aggregation` on the resource
2. **Kind-level ConfigMap** - `resource.customizations.health-aggregation.<group>_<kind>` in argocd-cm
3. **Default behavior** - Standard aggregation using the resource health

**Note**: When both annotation and Kind-level configuration exist, the mappings are merged with annotation values taking precedence for overlapping status keys. This allows fine-grained per-resource overrides while keeping other mappings from Kind-level config.

**Mapping Syntax**:

- **ConfigMap format**: `<SourceStatus>: <TargetStatus>` (colon separator, newline-separated for multiple mappings)
- **Annotation format**: `<SourceStatus>=<TargetStatus>` (equals separator, comma-separated for multiple mappings)
  - Single mapping: `Suspended=Healthy`
  - Multiple mappings: `Suspended=Healthy,Progressing=Healthy`
- Valid statuses: `Healthy`, `Progressing`, `Suspended`, `Degraded`, `Missing`, `Unknown`
- **Wildcard Support** (ConfigMap only): Use underscore `_` as wildcard character (same as `resource.customizations.health`)
  - `batch_*` matches `batch_Job`, `batch_CronJob`, etc.
  - `*_Job` matches any group with Kind `Job`
  - `custom.io_*` matches all Kinds in `custom.io` group
  - Follows the same pattern as the `resource_customizations/` directory structure
- **To ignore resources**: Use the existing `argocd.argoproj.io/ignore-healthcheck: "true"` annotation (no special mapping syntax needed)

#### Code Changes

**1. New Constants** (`common/common.go`):

```go
// AnnotationHealthAggregation allows overriding how a resource's health status is aggregated
AnnotationHealthAggregation = "argocd.argoproj.io/health-aggregation"
```

**2. Extend ResourceOverride** (`pkg/apis/application/v1alpha1/types.go`):

```go
type ResourceOverride struct {
    HealthLua             string                 `protobuf:"bytes,1,opt,name=healthLua"`
    UseOpenLibs           bool                   `protobuf:"bytes,5,opt,name=useOpenLibs"`
    Actions               string                 `protobuf:"bytes,3,opt,name=actions"`
    IgnoreDifferences     OverrideIgnoreDiff     `protobuf:"bytes,2,opt,name=ignoreDifferences"`
    IgnoreResourceUpdates OverrideIgnoreDiff     `protobuf:"bytes,6,opt,name=ignoreResourceUpdates"`
    KnownTypeFields       []KnownTypeField       `protobuf:"bytes,4,opt,name=knownTypeFields"`
    // NEW: Health aggregation mapping
    HealthAggregation  map[string]string      `protobuf:"bytes,7,opt,name=healthAggregation"`
}
```

**3. Core Aggregation Logic** (`controller/health.go`):

```go
func setApplicationHealth(resources []managedResource, statuses []appv1.ResourceStatus, resourceOverrides map[string]appv1.ResourceOverride, app *appv1.Application, persistResourceHealth bool) (health.HealthStatusCode, error) {
    var savedErr error
    var errCount uint

    appHealthStatus := health.HealthStatusHealthy
    for i, res := range resources {
        // ... existing skip logic ...

        if res.Live != nil && res.Live.GetAnnotations() != nil && res.Live.GetAnnotations()[common.AnnotationIgnoreHealthCheck] == "true" {
            continue
        }

        // ... compute healthStatus ...

        if healthStatus == nil {
            continue
        }

        // NEW: Build health aggregation mapping with proper precedence
        // Precedence: Per-resource annotation > Kind-level ConfigMap > default behavior
        finalMapping := make(map[string]string)

        // Step 1: Get Kind-level configuration (base layer)
        gvk := schema.GroupVersionKind{Group: res.Group, Version: res.Version, Kind: res.Kind}
        if kindMapping := settings.GetHealthAggregationMapping(gvk, resourceOverrides); len(kindMapping) > 0 {
            for k, v := range kindMapping {
                finalMapping[k] = v
            }
        }

        // Step 2: Merge per-resource annotation (override layer)
        if res.Live != nil && res.Live.GetAnnotations() != nil {
            if mapStr, ok := res.Live.GetAnnotations()[common.AnnotationHealthAggregation]; ok {
                if annotationMapping, err := parseHealthAggregationAnnotation(mapStr); err == nil {
                    // Annotation mappings override Kind-level mappings
                    for k, v := range annotationMapping {
                        finalMapping[k] = v
                    }
                }
            }
        }

        // Step 3: Apply the final merged mapping
        aggregatedStatus := applyHealthMapping(healthStatus.Status, finalMapping)

        // ... persist health status ...

        // Use aggregated status for comparison
        if health.IsWorse(appHealthStatus, aggregatedStatus) {
            appHealthStatus = aggregatedStatus
        }
    }

    // ... rest of function ...
}

// applyHealthMapping applies the health status mapping
// Returns the mapped status, or the original status if no mapping found
func applyHealthMapping(status health.HealthStatusCode, mapping map[string]string) health.HealthStatusCode {
    // Check for specific status mapping
    if mapped, ok := mapping[string(status)]; ok {
        return health.HealthStatusCode(mapped)
    }

    // No mapping found, return original
    // Note: To ignore resources entirely, use argocd.argoproj.io/ignore-healthcheck annotation
    return status
}
```

**4. Wildcard Matching**:

```go
// GetHealthAggregationMapping returns the health aggregation mapping for a GVK
// Supports wildcard matching using underscore '_' character (same as resource.customizations.health)
func GetHealthAggregationMapping(gvk schema.GroupVersionKind, overrides map[string]appv1.ResourceOverride) map[string]string {
    key := GetConfigMapKey(gvk)

    // Try exact match first
    if override, ok := overrides[key]; ok && len(override.HealthAggregation) > 0 {
        return override.HealthAggregation
    }

    // Try wildcard matches (same logic as resource.customizations.health)
    // This uses the doublestar library pattern matching
    for wildcardKey, override := range overrides {
        if len(override.HealthAggregation) > 0 && matchesWildcard(key, wildcardKey) {
            return override.HealthAggregation
        }
    }

    return nil
}

// matchesWildcard checks if a key matches a wildcard pattern
// Uses underscore '_' as wildcard character
func matchesWildcard(key, pattern string) bool {
    // Convert underscore to asterisk for glob matching
    globPattern := strings.ReplaceAll(pattern, "_", "*")
    matched, _ := doublestar.Match(globPattern, key)
    return matched
}
```

### Detailed Examples

#### Example 1: Default Configuration

Argo CD can ship with these defaults in the base `argocd-cm` to address #19126 and #24428.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  # Treat suspended Jobs as healthy (addresses #19126)
  resource.customizations.health-aggregation.batch_Job: |
    Suspended: Healthy

  # Treat suspended CronJobs as healthy (addresses #19126 and #24428)
  resource.customizations.health-aggregation.batch_CronJob: |
    Suspended: Healthy
```

**To restore original behavior** (if needed):

Remove the config map key or set to empty

```yaml
data:
  # Empty value restores original behavior
  resource.customizations.health-aggregation.batch_Job: ''
  resource.customizations.health-aggregation.batch_CronJob: ''
```

#### Example 2: Per-Resource Override (Takes Precedence)

User wants a specific Job to use original behavior (suspended = suspended), overriding the default:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: important-job
  annotations:
    # Override the default: map Suspended to Suspended instead of Healthy
    argocd.argoproj.io/health-aggregation: 'Suspended=Suspended'
spec:
  suspend: true
  # ... job spec
```

Or, user wants a specific Job to be completely ignored from health aggregation:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: optional-job
  annotations:
    # Use existing ignore-healthcheck annotation
    argocd.argoproj.io/ignore-healthcheck: 'true'
spec:
  # ... job spec
```

#### Example 3: Wildcard Matching

Map statuses for all resources in a group using wildcard:

```yaml
data:
  # Apply to all Kinds in custom.io group
  resource.customizations.health-aggregation.custom.io_*: |
    Suspended: Healthy
    Progressing: Healthy

  # Apply to all Job types regardless of group
  resource.customizations.health-aggregation.*_Job: |
    Suspended: Healthy

  # Apply to all batch resources
  resource.customizations.health-aggregation.batch_*: |
    Suspended: Healthy
```

#### Example 4: Ignoring Resources

To completely ignore resources from health aggregation, use the existing annotation.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dynamic-config
  annotations:
    # This ConfigMap won't affect Application health
    argocd.argoproj.io/ignore-healthcheck: 'true'
data:
  key: value
```

---

## Alternative 2: Lua-Based Health Aggregation (Recommended)

This approach extends the existing Lua health check mechanism to allow scripts to return a separate `aggregationHealth` field. This is **simpler, more consistent, and solves the open question** about providing defaults via `resource_customizations/`.

### Key Advantages

1. **No new ConfigMap keys needed** - reuses existing `resource.customizations.health.<group>_<kind>` pattern
2. **Defaults can be shipped in `resource_customizations/` folder** - same as existing health checks
3. **More flexible** - Lua scripts can implement conditional logic if needed in the future
4. **Consistent with existing patterns** - developers already know how to customize health checks
5. **Simpler configuration** - one mechanism instead of two

### Configuration Format

**Per-Resource Annotation** (highest precedence, same as Alternative 1):

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: manual-backup
  annotations:
    # Single mapping
    argocd.argoproj.io/health-aggregation: 'Suspended=Healthy'
    # Or multiple mappings (comma-separated)
    # argocd.argoproj.io/health-aggregation: 'Suspended=Healthy,Progressing=Healthy'
spec:
  suspend: true
  # ... rest of spec
```

**Kind-Level Configuration via Lua Health Check**:

Instead of a new ConfigMap key, extend existing health Lua scripts to return `aggregationHealth`:

```lua
-- resource_customizations/batch/Job/health.lua
hs = {}
if obj.status ~= nil then
  if obj.status.succeeded ~= nil and obj.status.succeeded > 0 then
    hs.status = "Healthy"
  elseif obj.spec.suspend ~= nil and obj.spec.suspend == true then
    hs.status = "Suspended"
    -- NEW: Override aggregation health
    hs.aggregationHealth = "Healthy"
  elseif obj.status.failed ~= nil and obj.status.failed > 0 then
    hs.status = "Degraded"
  else
    hs.status = "Progressing"
  end
end
return hs
```

**Configuration Precedence** (highest to lowest):

1. **Per-resource annotation** - `argocd.argoproj.io/health-aggregation` on the resource
2. **Lua script `aggregationHealth`** - returned by custom health check script
3. **Default behavior** - Use `hs.status` for aggregation (backward compatible)

### Code Changes

**1. Extend Health Status Struct** (`util/health/health.go`):

```go
type HealthStatus struct {
    Status  HealthStatusCode `json:"status,omitempty"`
    Message string           `json:"message,omitempty"`
    // NEW: Optional override for aggregation purposes
    AggregationHealth HealthStatusCode `json:"aggregationHealth,omitempty"`
}
```

**2. Lua Health Check Execution** (`util/lua/lua.go`):

Modify the Lua health check execution to extract the optional `aggregationHealth` field:

```go
func (vm VM) ExecuteHealthLua(obj *unstructured.Unstructured, script string) (*health.HealthStatus, error) {
    // ... existing Lua execution code ...

    // Extract status (existing)
    status, err := vm.GetField("status", lua.LTString)
    if err != nil {
        return nil, err
    }

    // Extract message (existing)
    message, _ := vm.GetField("message", lua.LTString)

    // NEW: Extract optional aggregationHealth
    aggregationHealth, _ := vm.GetField("aggregationHealth", lua.LTString)

    healthStatus := &health.HealthStatus{
        Status:  health.HealthStatusCode(status),
        Message: message,
        AggregationHealth:  health.HealthStatusCode(status),
    }

    // NEW: Set aggregation health if provided
    if aggregationHealth != "" {
        healthStatus.AggregationHealth = health.HealthStatusCode(aggregationHealth)
    }

    return healthStatus, nil
}
```

**3. Core Aggregation Logic** (`controller/health.go`):

```go
func setApplicationHealth(resources []managedResource, statuses []appv1.ResourceStatus, resourceOverrides map[string]appv1.ResourceOverride, app *appv1.Application, persistResourceHealth bool) (health.HealthStatusCode, error) {
    var savedErr error
    var errCount uint

    appHealthStatus := health.HealthStatusHealthy
    for i, res := range resources {
        // ... existing skip logic ...

        if res.Live != nil && res.Live.GetAnnotations() != nil && res.Live.GetAnnotations()[common.AnnotationIgnoreHealthCheck] == "true" {
            continue
        }

        // ... compute healthStatus ...

        if healthStatus == nil {
            continue
        }

        // NEW: Determine aggregation health with proper precedence
        aggregationStatus := healthStatus.AggregationHealth // Default: use status

        // Step 1: Check for per-resource annotation override (highest precedence)
        if res.Live != nil && res.Live.GetAnnotations() != nil {
            if mapStr, ok := res.Live.GetAnnotations()[common.AnnotationHealthAggregation]; ok {
                if annotationMapping, err := parseHealthAggregationAnnotation(mapStr); err == nil {
                    // Apply annotation mapping to the current aggregation status
                    if mapped, ok := annotationMapping[string(aggregationStatus)]; ok {
                        aggregationStatus = health.HealthStatusCode(mapped)
                    }
                }
            }
        }

        // ... persist health status (use original healthStatus.Status) ...

        // Use aggregation status for comparison
        if health.IsWorse(appHealthStatus, aggregationStatus) {
            appHealthStatus = aggregationStatus
        }
    }

    // ... rest of function ...
}
```

### Detailed Examples

#### Example 1: Default Configuration via resource_customizations/

Argo CD ships with built-in health checks that set `aggregationHealth`:

**File: `resource_customizations/batch/Job/health.lua`**

```lua
hs = {}
if obj.status ~= nil then
  if obj.status.succeeded ~= nil and obj.status.succeeded > 0 then
    hs.status = "Healthy"
  elseif obj.spec.suspend ~= nil and obj.spec.suspend == true then
    hs.status = "Suspended"
    hs.aggregationHealth = "Healthy"  -- Suspended Jobs don't affect app health
  elseif obj.status.failed ~= nil and obj.status.failed > 0 then
    hs.status = "Degraded"
  else
    hs.status = "Progressing"
  end
end
return hs
```

**File: `resource_customizations/batch/CronJob/health.lua`**

```lua
hs = {}
if obj.spec.suspend ~= nil and obj.spec.suspend == true then
  hs.status = "Suspended"
  hs.aggregationHealth = "Healthy"  -- Suspended CronJobs don't affect app health
else
  hs.status = "Healthy"
end
return hs
```

**To restore original behavior**: Users can override with their own Lua script that doesn't set `aggregationHealth`.

#### Example 2: Per-Resource Annotation Override

Same as Alternative 1 - annotation overrides Lua-provided `aggregationHealth`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: important-job
  annotations:
    # Override: this Job should affect app health even when suspended
    argocd.argoproj.io/health-aggregation: 'Suspended=Suspended'
spec:
  suspend: true
  # ... job spec
```

#### Example 3: Custom Resource with Conditional Logic

For more complex scenarios, Lua can implement conditional logic:

```lua
hs = {}
if obj.status ~= nil then
  if obj.status.phase == "Paused" then
    hs.status = "Suspended"
    -- Only treat as healthy if paused by operator (has annotation)
    if obj.metadata.annotations ~= nil and obj.metadata.annotations["paused-by"] == "operator" then
      hs.aggregationHealth = "Healthy"
    end
  elseif obj.status.phase == "Running" then
    hs.status = "Healthy"
  end
end
return hs
```

---

## Alternative 3: Custom Health Check Override (Does Not Meet Requirements)

This approach uses the existing custom health check mechanism to remap health statuses.

### How It Works

Users can already override health checks using custom Lua scripts in the ConfigMap or via the `resource.customizations.health.<group>_<kind>` key. To "solve" the suspended CronJob problem, a user could write:

```lua
-- Custom health check that treats suspended as healthy
hs = {}
if obj.spec.suspend ~= nil and obj.spec.suspend == true then
  hs.status = "Healthy"  -- Remap Suspended to Healthy
  hs.message = "CronJob is suspended"
else
  hs.status = "Healthy"
end
return hs
```

Or, for a more complex case with Jobs:

```lua
hs = {}
if obj.status ~= nil then
  if obj.status.succeeded ~= nil and obj.status.succeeded > 0 then
    hs.status = "Healthy"
  elseif obj.spec.suspend ~= nil and obj.spec.suspend == true then
    hs.status = "Healthy"  -- Remap Suspended to Healthy
    hs.message = "Job is suspended"
  elseif obj.status.failed ~= nil and obj.status.failed > 0 then
    hs.status = "Degraded"
  else
    hs.status = "Progressing"
  end
end
return hs
```

### Per-Resource Override with Annotation

To enable per-resource overrides, users could add an annotation-based custom health check:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup-job
  annotations:
    # Use existing ignore-healthcheck annotation
    argocd.argoproj.io/ignore-healthcheck: 'true'
spec:
  suspend: true
  # ... rest of spec
```

Or use a custom health check that reads annotations to decide behavior.

### Critical Limitation: Does Not Satisfy R1

**This approach fundamentally fails to meet Requirement R1** because it remaps the resource's actual health status. This means:

❌ **Loss of Resource State Visibility**: The resource tree will show "Healthy" (with a green heart icon) instead of "Suspended" (with the appropriate suspended icon). Operators lose visibility into the actual state of the resource.

❌ **Misleading Resource Information**: A suspended CronJob showing as "Healthy" gives the false impression that the CronJob is running normally, when it's actually suspended.

❌ **No Distinction Between Actual and Aggregated Health**: The resource's displayed health is the same as what contributes to aggregation. There's no way to show "this resource is Suspended, but that's okay for the Application."

❌ **Debugging Difficulty**: When troubleshooting, operators cannot distinguish between resources that are genuinely healthy and resources that are in a non-healthy state but have been remapped.

### Example Comparison

Consider a suspended CronJob:

| Approach               | Resource Tree Display | Application Health | Satisfies R1?                                    |
| ---------------------- | --------------------- | ------------------ | ------------------------------------------------ |
| **Current Behavior**   | Suspended (⏸️ icon)   | Suspended          | ❌ No (Application incorrectly marked Suspended) |
| **Alternative 3**      | Healthy (❤️ icon)     | Healthy            | ❌ No (Resource state is hidden)                 |
| **Alternative 1 or 2** | Suspended (⏸️ icon)   | Healthy            | ✅ Yes (Distinct states preserved)               |

### Why This Matters: Real-World Scenarios

**Scenario 1: Monitoring and Alerting**

An operator is investigating why an Application is not behaving as expected. They open the resource tree:

- **With Alternative 3**: All resources show as "Healthy". The operator doesn't realize that several CronJobs are suspended and not running their scheduled tasks.
- **With Alternative 1/2**: Resources correctly show as "Suspended". The operator immediately sees which CronJobs are not running and can investigate if this is intentional or a problem.

**Scenario 2: Operational Visibility**

A team wants to see which CronJobs are currently suspended for maintenance:

- **With Alternative 3**: Impossible to determine from the UI. The health status has been remapped to "Healthy", hiding the actual state.
- **With Alternative 1/2**: Suspended CronJobs are clearly visible in the resource tree with the suspended icon, while the Application remains healthy.

**Scenario 3: Debugging a Broken Job**

A Job is suspended but also has failures in its status:

- **With Alternative 3**: The custom health check might show "Healthy" (hiding both the suspension and the failures), or it might show "Degraded" (but you can't tell if it's suspended or not).
- **With Alternative 1/2**: The Job shows its actual health state (e.g., "Degraded" with details about failures), and the aggregation behavior can be configured independently.

### Additional Drawbacks

Beyond failing R1, this approach has other significant drawbacks:

1. **No Default Behavior (R2)**: Cannot ship sensible defaults in `resource_customizations/` without changing the displayed health status for all users. This would be a breaking change.

2. **Requires Overriding Built-in Health Checks**: Users must completely replace the built-in health check logic, which means:
   - Losing future improvements to built-in health checks
   - Maintaining custom Lua code for every resource type
   - Risk of bugs in custom health check logic

3. **Cannot Conditionally Apply**: If you want some suspended Jobs to affect health and others not to, you need complex Lua logic that reads annotations, making the health check script very complicated.

4. **Ignoring Resources Too Broad**: Using `argocd.argoproj.io/ignore-healthcheck: "true"` completely removes the resource from health calculation, which means:
   - The resource won't affect health even if it becomes Degraded
   - No visibility into the resource's health at all
   - Cannot distinguish between "intentionally suspended" and "broken"

### Conclusion: Why Alternative 3 Is Insufficient

While Alternative 3 (custom health check override) is technically possible with existing Argo CD functionality, it **fundamentally fails to meet the core requirement** of maintaining distinct health states for resource display and aggregation. This limitation makes it unsuitable for the use cases described in this proposal.

The value of Alternatives 1 and 2 is precisely that they **preserve resource state visibility** while allowing flexible aggregation behavior. This is not achievable through custom health checks alone.

---

### Comparison of All Alternatives

| Aspect                             | Alternative 1 (ConfigMap)                              | Alternative 2 (Lua)                                                | Alternative 3 (Custom Health Check)                     |
| ---------------------------------- | ------------------------------------------------------ | ------------------------------------------------------------------ | ------------------------------------------------------- |
| **Satisfies R1** (Distinct States) | ✅ Yes                                                 | ✅ Yes                                                             | ❌ No                                                   |
| **Configuration location**         | New `resource.customizations.health-aggregation.*` key | Existing `resource.customizations.health.*` Lua scripts            | Existing `resource.customizations.health.*` Lua scripts |
| **Default configuration**          | Must be in argocd-cm ConfigMap                         | Can be in `resource_customizations/` folder (shipped with Argo CD) | Cannot ship without breaking changes                    |
| **Flexibility**                    | Simple string mapping only                             | Can include conditional logic                                      | Full Lua flexibility                                    |
| **Learning curve**                 | New mechanism to learn                                 | Reuses existing health check pattern                               | Reuses existing health check pattern                    |
| **Wildcard support**               | Yes, via ConfigMap keys                                | Yes, via file structure (same as health checks)                    | Yes, via ConfigMap keys                                 |
| **Annotation override**            | Yes                                                    | Yes                                                                | Via ignore-healthcheck only                             |
| **Code complexity**                | Medium (new parsing, wildcard matching)                | Low (extend existing Lua execution)                                | None (already exists)                                   |
| **Resource Tree Accuracy**         | ✅ Shows actual health                                 | ✅ Shows actual health                                             | ❌ Shows remapped health                                |
| **Operational Visibility**         | ✅ Full visibility                                     | ✅ Full visibility                                                 | ❌ Hidden state                                         |

### Security Considerations

**Alternative 1 (ConfigMap-based)**:

- **No new security risks**: This feature only affects how health statuses are aggregated, not how they're calculated
- **Annotation access**: Users with permission to modify resources can add per-resource overrides
- **No code execution**: Uses simple string mapping with no script execution
- **Validation**: Invalid mappings should be rejected with clear error messages

**Alternative 2 (Lua-based)**:

- **Reuses existing security model**: Lua health checks are already part of Argo CD
- **No additional code execution**: Only extends existing Lua health check mechanism
- **Same trust model**: Lua scripts in `resource_customizations/` are trusted (shipped with Argo CD or configured by admins)
- **Annotation access**: Users with permission to modify resources can add per-resource overrides

### Risks and Mitigations

**Risk 1: Users misconfigure and hide real health issues**

Currently, the Application only has an aggregated health, without details on which resource caused the
Application to be in that state. Additional information was not needed in the UI since it was straightforward
to find out why. With aggregation health overrides, this might be more complex and require more details

- _Mitigation_: Resource health is still visible in UI, only aggregation is affected

**Risk 2: Configuration complexity and user confusion**

- _Mitigation_: Provide clear examples in documentation

**Risk 3: Performance impact of parsing annotations on every health check**

- _Mitigation_: Annotation parsing is simple string operations, minimal overhead

**Risk 4 (Alternative 2 only): Lua scripts become more complex**

- _Mitigation_: `aggregationHealth` is optional; most scripts won't need it. Clear documentation and examples provided.

## Drawbacks

**Alternative 1 (ConfigMap-based)**:

1. **Additional configuration surface**: Users need to learn a new configuration mechanism
2. **Separate from health checks**: Health calculation and aggregation are configured in different places
3. **Defaults require ConfigMap**: Cannot ship defaults in `resource_customizations/` folder

**Alternative 2 (Lua-based)**:

1. **Requires Lua knowledge**: Users need to understand Lua to customize (but this is already true for health checks)
2. **More complex for simple mappings**: Simple status mappings require Lua script instead of YAML config

**Both alternatives**:

1. **Potential for misconfiguration**: Users might hide real health issues by misconfiguring mappings
2. **Breaking change risk**: If we ship with new defaults, some users may be surprised

## Implementation Plan

### Alternative 1: ConfigMap-Based Implementation

#### Phase 1: Core Implementation

1. Add `HealthAggregation` field to `ResourceOverride` type in `pkg/apis/application/v1alpha1/types.go`
2. Add `AnnotationHealthAggregation` constant in `common/common.go`
3. Implement parsing functions in `util/settings/settings.go`:
   - `parseHealthAggregation()` for ConfigMap format (colon separator, newline-separated)
   - Update `GetResourceOverrides()` to parse `resource.customizations.health-aggregation.*` fields
4. Implement parsing function in `controller/health.go`:
   - `parseHealthAggregationAnnotation()` for annotation format (equals separator, comma-separated)
5. Implement wildcard matching based on `util/lua/lua.go`:
   - `GetHealthAggregationMapping()` function with wildcard support (using doublestar library)
   - `matchesWildcard()` helper function
6. Modify `setApplicationHealth()` in `controller/health.go` to:
   - Build merged mapping from Kind-level config and per-resource annotation
   - Apply mapping with proper precedence (annotation overrides Kind-level)
   - Use single `applyHealthMapping()` call with final merged map
7. Unit tests for all new functions

#### Phase 2: Default Configuration

1. Add default mappings for Job and CronJob to base ConfigMap manifest in `manifests/base/config-cm.yaml`
2. Update installation manifests (install.yaml, namespace-install.yaml, etc.)
3. E2E test

#### Phase 3: Documentation

1. Update `docs/operator-manual/health.md` with new section on health aggregation customization
2. Add examples for common use cases
3. Document precedence order clearly

---

### Alternative 2: Lua-Based Implementation (Recommended)

#### Phase 1: Core Implementation

1. Add `AggregationHealth` field to `HealthStatus` struct in `util/health/health.go`
2. Add `AnnotationHealthAggregation` constant in `common/common.go`
3. Modify `ExecuteHealthLua()` in `util/lua/lua.go`:
   - Extract optional `aggregationHealth` field from Lua return value
   - Set `HealthStatus.AggregationHealth` if provided
4. Implement parsing function in `controller/health.go`:
   - `parseHealthAggregationAnnotation()` for annotation format (equals separator, comma-separated)
5. Modify `setApplicationHealth()` in `controller/health.go`:
   - Use `healthStatus.AggregationHealth` if set, otherwise use `healthStatus.Status`
   - Apply annotation override if present (highest precedence)
6. Unit tests:
   - Lua script returning `aggregationHealth`
   - Annotation parsing and override behavior
   - Precedence: annotation > Lua aggregationHealth > status

#### Phase 2: Default Configuration

1. Update built-in health checks in `resource_customizations/`:
   - `resource_customizations/batch/Job/health.lua` - add `hs.aggregationHealth = "Healthy"` when suspended
   - `resource_customizations/batch/CronJob/health.lua` - add `hs.aggregationHealth = "Healthy"` when suspended
2. E2E tests

#### Phase 3: Documentation

1. Update `docs/operator-manual/health.md`:
   - Document `aggregationHealth` field in Lua health checks
   - Add examples for common use cases
   - Document precedence order clearly
2. Add migration guide for users with custom health checks

## Summary of Key Decisions

1. ✅ **Three alternatives evaluated**: ConfigMap-based (Alternative 1), Lua-based (Alternative 2, recommended), and Custom Health Check Override (Alternative 3, insufficient)
2. ✅ **Critical requirement identified (R1)**: Must maintain distinct health states for resource display and aggregation
3. ✅ **Alternative 3 rejected**: Does not satisfy R1 - remaps resource health status, hiding actual state from operators
4. ✅ **Ship with defaults**: Job and CronJob get suspended → healthy mapping by default
5. ✅ **Breaking change accepted**: This fixes incorrect behavior per community feedback in #19126
6. ✅ **Ignoring resources**: Use existing `argocd.argoproj.io/ignore-healthcheck` annotation (no special mapping syntax)
7. ✅ **Clear precedence**: Annotation > Lua/ConfigMap > Default status (in that order)
8. ✅ **Downgrade safe**: No data loss, behavior simply reverts to original
9. ✅ **Consistent with existing patterns**: Alternative 2 reuses existing health check mechanism

### Recommendation: Alternative 2 (Lua-based)

**Rationale**:

- **Satisfies all requirements (R1-R5)**: Particularly R1 (distinct health states) and R2 (default configuration)
- **Solves the open question**: Defaults can be shipped in `resource_customizations/` folder
- **Simpler architecture**: Extends existing mechanism instead of adding new ConfigMap keys
- **More flexible**: Lua can implement conditional logic if needed
- **Better developer experience**: One place to configure both health calculation and aggregation
- **Easier to maintain**: Built-in health checks are versioned with the codebase
- **Preserves operational visibility**: Resource tree shows actual health state while Application uses aggregated health

**Trade-off**: Requires Lua knowledge for customization, but this is already required for custom health checks.

**Why Not Alternative 3**: While Alternative 3 (custom health check override) requires no new code, it fundamentally fails to meet the core requirement (R1) of maintaining distinct health states for resource display and aggregation. This makes it unsuitable for the use cases described in this proposal, where operators need to see the actual resource state while having different aggregation behavior.

## Frequently Asked Questions

### Q: Why not just use custom health checks to remap statuses?

**A**: Custom health checks remap the resource's displayed health status, which means operators lose visibility into the actual state of the resource. This proposal maintains **two separate health states**: one for display (actual resource state) and one for aggregation (contribution to Application health). This distinction is critical for operational visibility and debugging.

See **Alternative 3** section for detailed examples of why custom health checks are insufficient.

### Q: Can't users just add `argocd.argoproj.io/ignore-healthcheck: "true"` to resources they want to ignore?

**A**: The `ignore-healthcheck` annotation completely removes the resource from health calculation, which means:

- The resource won't affect Application health even if it becomes Degraded or fails
- No visibility into the resource's health at all in some contexts
- Cannot distinguish between "intentionally suspended" and "broken"

This proposal provides more nuanced control: a suspended CronJob can still affect Application health if it becomes Degraded, but suspension itself is treated as healthy.

### Q: Is this just about CronJobs and Jobs?

**A**: While suspended Jobs/CronJobs are the most common use case (and the motivation from issues #19126, #24428), this proposal solves a broader problem:

- HPAs showing as "Progressing" during normal scale operations
- Custom resources with domain-specific health states
- Any scenario where a resource's health state has different meanings for display vs aggregation

The goal is to provide a general mechanism that works for any resource type.

### Q: Won't this hide real health issues?

**A**: No, because:

1. **Resource health is still visible**: The resource tree shows the actual health state with the correct icon
2. **Aggregation is configurable**: Users explicitly choose which statuses to remap
3. **Degraded states still bubble up**: By default, only specific states (like Suspended) are remapped; Degraded and other failure states still affect Application health

Example: A suspended CronJob that also has failures would show as "Degraded" (not "Suspended"), and Degraded would still affect Application health unless explicitly configured otherwise.

### Q: What if I want to restore the original behavior?

**A**: Easy:

- **Alternative 1**: Remove or empty the ConfigMap key for the resource type
- **Alternative 2**: Override the built-in Lua script with one that doesn't set `aggregationHealth`
- **Per-resource**: Add annotation to override the default behavior

The feature is designed to be fully reversible without data loss.

### Q: How does this affect performance?

**A**: Minimal impact:

- **Alternative 1**: Simple string map lookup during health aggregation (already happening)
- **Alternative 2**: Lua scripts already execute for health checks; just extracts one additional field
- **Annotation parsing**: Simple string operations, only when annotation is present

Health aggregation is not a hot path, so the additional overhead is negligible.

### Q: Will this work with existing custom health checks?

**A**: Yes:

- **Alternative 1**: Works independently of health checks; operates on the health status after it's calculated
- **Alternative 2**: Extends existing health checks with an optional field; existing health checks continue to work without changes

### Q: What about backward compatibility?

**A**: Fully backward compatible:

- Existing Applications continue to work without changes
- The feature is opt-in via configuration
- If we ship with defaults for Jobs/CronJobs, users can override or disable them

The only "breaking change" is fixing the incorrect behavior where suspended Jobs/CronJobs mark Applications as Suspended, which is widely considered a bug per community feedback.

### Q: Can I have different behavior for different instances of the same Kind?

**A**: Yes, using per-resource annotations:

- Set a Kind-level default in ConfigMap or Lua script
- Override specific instances with `argocd.argoproj.io/health-aggregation` annotation
- Annotation takes precedence over Kind-level configuration

See **Use Case 4** and **Use Case 5** for examples.

### Q: How does this interact with `ignore-healthcheck`?

**A**: They work together:

- `ignore-healthcheck: "true"` completely removes the resource from health calculation (existing behavior)
- Health aggregation overrides (this proposal) allow the resource to participate in health calculation but with remapped status
- If both are present, `ignore-healthcheck` takes precedence (resource is completely ignored)

### Q: What happens if I configure an invalid mapping?

**A**:

- Invalid status names should be rejected with clear error messages
- Invalid mappings are logged and ignored (fall back to default behavior)
- Application health calculation continues to work (fail-safe)

### Q: Can I use wildcards to match multiple resource types?

**A**:

- **Alternative 1**: Yes, using underscore `_` as wildcard in ConfigMap keys (e.g., `batch_*` matches all batch resources)
- **Alternative 2**: Yes, via file structure in `resource_customizations/` (same as existing health checks)

### Q: How do I know which health status a resource is contributing to the Application?

**A**: This is a UI/UX question that should be addressed in implementation:

- Resource tree could show both "Health: Suspended" and "Aggregates as: Healthy"
- Application health details could list which resources contributed to the aggregated health
- This is out of scope for this proposal but should be considered in implementation
