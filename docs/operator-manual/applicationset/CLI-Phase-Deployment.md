# ApplicationSet Phase Deployment CLI Guide

This guide demonstrates how to use the ArgoCD CLI to manage ApplicationSets with phase deployment strategies.

## Prerequisites

- ArgoCD CLI installed and configured
- ApplicationSet with phase deployment strategy configured
- Access to an ArgoCD server

## Command Overview

The ArgoCD CLI provides several commands for managing phase deployments:

- `argocd appset phase status` - Show current phase status
- `argocd appset phase advance` - Advance to next phase
- `argocd appset phase rollback` - Rollback to previous phase  
- `argocd appset phase history` - Show phase deployment history
- `argocd appset get --show-phase` - Show phase info in ApplicationSet details
- `argocd appset list` - List ApplicationSets with phase status (in wide output)

## Basic Usage Examples

### 1. Create an ApplicationSet with Phase Deployment

```bash
# Create an ApplicationSet with phase deployment strategy
# Use one of the existing examples from applicationset/examples/
argocd appset create applicationset/examples/phase-deployment-cli-demo.yaml

# Verify creation
argocd appset get phased-deployment-demo
```

### 2. Check Phase Status

```bash
# Show phase deployment status (table format)
argocd appset phase status phased-deployment-demo

# Show phase status in JSON format
argocd appset phase status phased-deployment-demo -o json

# Show phase status in YAML format  
argocd appset phase status phased-deployment-demo -o yaml
```

**Example Output:**
```
ApplicationSet: default/phased-deployment-demo
Phase Status: 1/3 (33.3% complete)

Phases:
PHASE  NAME         STATUS     TARGETS       CHECKS
0      development  Completed  1             1
1      staging      Current    1             1  
2      production   Pending    1             1
```

### 3. Advance to Next Phase

```bash
# Advance to the next phase
argocd appset phase advance phased-deployment-demo

# Dry run to see what would happen
argocd appset phase advance phased-deployment-demo --dry-run
```

**Example Output:**
```
ApplicationSet 'phased-deployment-demo' advanced to phase 2/3
```

### 4. Rollback to Previous Phase

```bash
# Rollback to previous phase
argocd appset phase rollback phased-deployment-demo

# Dry run rollback
argocd appset phase rollback phased-deployment-demo --dry-run
```

**Example Output:**
```
ApplicationSet 'phased-deployment-demo' rolled back to phase 1 (from phase 2)
```

### 5. View Phase History

```bash
# Show phase deployment history
argocd appset phase history phased-deployment-demo

# Show history in JSON format
argocd appset phase history phased-deployment-demo -o json
```

**Example Output:**
```
ApplicationSet: default/phased-deployment-demo

Phase History:
TYPE     VALUE  TIMESTAMP
CURRENT  2      2023-01-01 15:04:05
ROLLBACK 3      2023-01-01T15:05:00Z
```

### 6. Enhanced ApplicationSet Details

```bash
# Get ApplicationSet details with automatic phase info (if configured)
argocd appset get phased-deployment-demo

# Explicitly show phase information
argocd appset get phased-deployment-demo --show-phase

# Show ApplicationSet with parameters and phase info
argocd appset get phased-deployment-demo --show-params --show-phase
```

### 7. List ApplicationSets with Phase Information

```bash
# List ApplicationSets (shows phase status in wide output)
argocd appset list

# List in wide format (includes PHASE column)
argocd appset list -o wide
```

**Example Output:**
```
NAME                    PROJECT  SYNCPOLICY  CONDITIONS  REPO                          PATH                  TARGET  PHASE
phased-deployment-demo  default  <none>      []          https://github.com/example/  overlays/{{env}}     HEAD    2/3
```

## JSON/YAML Output Examples

### Phase Status JSON Output

```bash
argocd appset phase status phased-deployment-demo -o json
```

```json
{
  "applicationSetName": "default/phased-deployment-demo",
  "currentPhase": 1,
  "totalPhases": 3,
  "completed": false,
  "percentage": 33.3,
  "phases": [
    {
      "index": 0,
      "name": "development",
      "status": "completed",
      "targets": 1,
      "checks": 1
    },
    {
      "index": 1,
      "name": "staging", 
      "status": "current",
      "targets": 1,
      "checks": 1
    },
    {
      "index": 2,
      "name": "production",
      "status": "pending", 
      "targets": 1,
      "checks": 1
    }
  ]
}
```

### Phase History JSON Output

```bash
argocd appset phase history phased-deployment-demo -o json
```

```json
{
  "applicationSetName": "default/phased-deployment-demo",
  "events": [
    {
      "type": "current",
      "value": "1",
      "timestamp": "2023-01-01T15:04:05Z"
    },
    {
      "type": "rollback", 
      "value": "2",
      "timestamp": "2023-01-01T15:05:00Z"
    }
  ],
  "annotations": {
    "applicationset.argoproj.io/phase-clusters": "1",
    "applicationset.argoproj.io/rollback-phase-clusters": "2"
  }
}
```

## Common Workflows

### 1. Monitoring Phase Deployment Progress

```bash
# Check status
argocd appset phase status my-phased-app

# Wait for phase to complete, then advance
argocd appset phase advance my-phased-app

# Check if advancement was successful
argocd appset phase status my-phased-app
```

### 2. Emergency Rollback

```bash
# Check current status
argocd appset phase status my-phased-app

# Rollback if issues detected
argocd appset phase rollback my-phased-app

# Verify rollback
argocd appset phase status my-phased-app
argocd appset phase history my-phased-app
```

### 3. Automation Scripts

```bash
#!/bin/bash
# Script to advance phase after validation

APP_SET="my-phased-app"

# Check current phase
CURRENT_PHASE=$(argocd appset phase status $APP_SET -o json | jq -r '.currentPhase')
TOTAL_PHASES=$(argocd appset phase status $APP_SET -o json | jq -r '.totalPhases')

echo "Current phase: $CURRENT_PHASE/$TOTAL_PHASES"

if [ $CURRENT_PHASE -lt $TOTAL_PHASES ]; then
    echo "Advancing to next phase..."
    argocd appset phase advance $APP_SET
    echo "Phase advanced successfully"
else
    echo "All phases completed"
fi
```

## Error Handling

### Common Error Scenarios

1. **ApplicationSet Not Found**
```bash
$ argocd appset phase status non-existent-app
Error: applicationset "non-existent-app" not found
```

2. **No Phase Deployment Strategy**
```bash
$ argocd appset phase status regular-appset
ApplicationSet: default/regular-appset
No phase deployment strategy configured
```

3. **Already at Final Phase**
```bash
$ argocd appset phase advance completed-app
ApplicationSet 'completed-app' has already completed all phases (3/3)
```

4. **Already at Initial Phase**
```bash
$ argocd appset phase rollback initial-app
ApplicationSet 'initial-app' is already at the initial phase (0)
```

## Integration with CI/CD

### GitLab CI Example

```yaml
deploy:
  stage: deploy
  script:
    - argocd appset create $APP_SET_MANIFEST
    - argocd appset phase status $APP_SET_NAME
  
advance_phase:
  stage: advance
  script:
    - argocd appset phase advance $APP_SET_NAME
    - argocd appset phase status $APP_SET_NAME
  when: manual
```

### GitHub Actions Example

```yaml
- name: Deploy ApplicationSet
  run: |
    argocd appset create applicationset.yaml
    argocd appset phase status my-app

- name: Advance Phase
  run: |
    argocd appset phase advance my-app
  if: github.event_name == 'workflow_dispatch'
```

## Best Practices

1. **Always check status before advancing**: Use `argocd appset phase status` before advancing phases

2. **Use dry-run for safety**: Test phase changes with `--dry-run` flag first

3. **Monitor via JSON output**: Use `-o json` for programmatic access in scripts

4. **Track history**: Regularly check `argocd appset phase history` for audit trail

5. **Automate with caution**: Be careful with automated phase advancement in production

6. **Use wide output**: List ApplicationSets with `-o wide` to see phase status overview

## Troubleshooting

### Debug Commands

```bash
# Get full ApplicationSet configuration
argocd appset get my-app -o yaml

# Check ApplicationSet events/status
argocd appset get my-app --show-phase

# View detailed phase history
argocd appset phase history my-app -o yaml

# Check ApplicationSet controller logs
kubectl logs -n argocd deployment/argocd-applicationset-controller
```

### Common Issues

1. **Phase not advancing automatically**: Check ApplicationSet controller logs and validation checks
2. **Annotations not updating**: Verify ApplicationSet update permissions
3. **Wrong phase count**: Ensure phase deployment strategy is correctly configured

This CLI interface provides complete control over ApplicationSet phase deployments, enabling both manual and automated management of phased rollouts.