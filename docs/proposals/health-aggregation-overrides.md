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
last-updated: 2026-01-15
---

# Health Aggregation Overrides

Introduce configurable health aggregation behavior to allow users to customize how resource health statuses are aggregated into Application health, addressing common use cases where default aggregation behavior doesn't match operational intent.

## Open Questions

~~Is there a way to provide sensible defaults that could be defined in the `resource_customizations/` folder instead of on the argocd-cm?~~

**Resolved**: Alternative 2 (Lua-based approach) solves this by allowing defaults to be shipped in `resource_customizations/` folder via Lua health check scripts.

## Summary

Currently, Argo CD aggregates resource health into Application health using a "worst health wins" algorithm. This causes issues when resources are intentionally in states like "Suspended" (e.g., suspended Jobs/CronJobs for DR scenarios) or when certain resource types shouldn't affect Application health (e.g., ConfigMaps). This proposal introduces annotation-based and ConfigMap-based configuration to customize health aggregation behavior per resource or per Kind.

**User Impact**: Users will be able to prevent suspended Jobs/CronJobs from marking their Applications as suspended, ignore certain resource types from health aggregation, and map resource health statuses to different values for aggregation purposes.

## Motivation

### Problems with Current Behavior

1. **Suspended Jobs/CronJobs** ([#19126](https://github.com/argoproj/argo-cd/issues/19126)): When Jobs or CronJobs have `spec.suspend: true` (introduced in K8s v1.24), the Application becomes "Suspended" even though this is intentional and desired behavior. This affects:

   - DR/emergency jobs deployed alongside services
   - Jobs managed by external controllers
   - Monitoring alerts that trigger incorrectly
   - Scripts that check app health

2. **Resources Without Health Significance**: Some resources don't have meaningful health at the Application level, but currently affect Application health, requiring an annotation on each resource.

3. **Operational Intent Mismatch**: Zero-replica Deployments, suspended CronJobs, and similar intentional states are treated as unhealthy when they represent valid operational states.

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

This proposal presents **two alternative approaches**:

1. **Alternative 1 (ConfigMap-based)**: Introduces a new ConfigMap key `resource.customizations.health-aggregation.<group>_<kind>` for simple status mappings
2. **Alternative 2 (Lua-based, recommended)**: Extends existing Lua health checks to return an optional `aggregationHealth` field

Both alternatives support per-resource annotation overrides. **Alternative 2 is recommended** because it:

- Reuses existing health check mechanism (simpler architecture)
- Allows defaults to be shipped in `resource_customizations/` folder
- Provides more flexibility for conditional logic
- Keeps health calculation and aggregation configuration in one place

See detailed comparison in the implementation sections below.

### Use Cases

#### Use Case 1: Suspended Jobs for DR Scenarios

As an operator, I deploy a MySQL database with a suspended backup Job. The Job is part of my Helm chart and can be manually triggered during DR scenarios. I want my Application to show as "Healthy" even though the Job is suspended, because suspension is intentional.

**Solution**: Configure Kind-level mapping for Jobs to treat Suspended as Healthy.

#### Use Case 2: Suspended CronJobs for Maintenance Windows

As a developer, I have CronJobs that run periodic tasks. During maintenance windows, I suspend these CronJobs. I don't want my Application to show as "Suspended" during this time.

**Solution**: Configure Kind-level mapping for CronJobs to treat Suspended as Healthy.

#### Use Case 3: Ignoring a specific Kind Health

As a platform engineer, I have custom resources that are dynamically created/deleted. I don't want missing these resources to affect Application health.

**Solution**: Add `argocd.argoproj.io/ignore-healthcheck: "true"` annotation to the resource, or configure them to have no health check in the first place.

#### Use Case 4: Per-Resource Override

As a developer, most of my Jobs should affect health normally, but one specific Job is a manual-trigger Job that should be treated as healthy when suspended.

**Solution**: Add annotation to that specific Job to override its health aggregation. The annotation will be merged with Kind-level configuration, with annotation values taking precedence.

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

### Comparison with Alternative 1

| Aspect                     | Alternative 1 (ConfigMap)                              | Alternative 2 (Lua)                                                |
| -------------------------- | ------------------------------------------------------ | ------------------------------------------------------------------ |
| **Configuration location** | New `resource.customizations.health-aggregation.*` key | Existing `resource.customizations.health.*` Lua scripts            |
| **Default configuration**  | Must be in argocd-cm ConfigMap                         | Can be in `resource_customizations/` folder (shipped with Argo CD) |
| **Flexibility**            | Simple string mapping only                             | Can include conditional logic                                      |
| **Learning curve**         | New mechanism to learn                                 | Reuses existing health check pattern                               |
| **Wildcard support**       | Yes, via ConfigMap keys                                | Yes, via file structure (same as health checks)                    |
| **Annotation override**    | Yes                                                    | Yes                                                                |
| **Code complexity**        | Medium (new parsing, wildcard matching)                | Low (extend existing Lua execution)                                |

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

1. ✅ **Two alternatives proposed**: ConfigMap-based (Alternative 1) vs Lua-based (Alternative 2, recommended)
2. ✅ **Ship with defaults**: Job and CronJob get suspended → healthy mapping by default
3. ✅ **Breaking change accepted**: This fixes incorrect behavior per community feedback in #19126
4. ✅ **Ignoring resources**: Use existing `argocd.argoproj.io/ignore-healthcheck` annotation (no special mapping syntax)
5. ✅ **Clear precedence**: Annotation > Lua/ConfigMap > Default status (in that order)
6. ✅ **Downgrade safe**: No data loss, behavior simply reverts to original
7. ✅ **Consistent with existing patterns**: Alternative 2 reuses existing health check mechanism

### Recommendation: Alternative 2 (Lua-based)

**Rationale**:

- **Solves the open question**: Defaults can be shipped in `resource_customizations/` folder
- **Simpler architecture**: Extends existing mechanism instead of adding new ConfigMap keys
- **More flexible**: Lua can implement conditional logic if needed
- **Better developer experience**: One place to configure both health calculation and aggregation
- **Easier to maintain**: Built-in health checks are versioned with the codebase

**Trade-off**: Requires Lua knowledge for customization, but this is already required for custom health checks.
