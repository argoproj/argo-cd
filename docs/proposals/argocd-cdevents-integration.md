---
title: ArgoCD to cdEvents Integration
authors:
  - @lukepatrick
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2025-06-21
last-updated: 2025-10-04
---


> ⚠️ **Proposal Idea Only**
>
> ---
>
> **This section is a *temporary idea space* and *not a formal proposal*.**  
> The concepts here have been debated elsewhere and are *not currently considered viable* in their present form.  
> For historical discussions and context, see: [argoproj/argo-cd#13723](https://github.com/argoproj/argo-cd/pull/13723).
>
> ---
>
> *Please treat this as exploratory thinking, not an actionable or recommended direction.*

# ArgoCD to cdEvents Integration

This proposal outlines the integration of ArgoCD's notification system with the cdEvents specification to enable standardized event emission for continuous deployment and operations activities.

## Open Questions

* Should cdEvents be enabled by default or as an opt-in feature?
* Should cdEvents interface via Notifications Engine/Controller or Core ArgoCD?
  * Existing discussions opt for Notifications Engine. 
  * I would be interested in how ArgoCD can react to a cdEvent message
* How should multi-application sync events be represented in cdEvents?
* Should we emit environment events for namespace lifecycle changes?

## Summary

ArgoCD currently provides a notification system that can send alerts to various destinations (Slack, email, webhooks) based on application lifecycle events. However, these notifications are ArgoCD-specific and may not easily integrate with broader CI/CD toolchain ecosystems.

This proposal extends ArgoCD's existing notification controller to emit cdEvents-compliant events alongside traditional notifications. cdEvents is a CDF specification for standardizing events across the software development lifecycle, enabling better interoperability between tools.

The design will allow ArgoCD to participate in interoperable CI/CD event ecosystems while maintaining backward compatibility with existing notification workflows.

## Motivation

The main motivation behind this enhancement proposal is to enable ArgoCD deployments to participate in a interoperable CI/CD event ecosystems, providing better observability, integration, and automation capabilities across the software development lifecycle.

Currently, ArgoCD's notifications are Argo-specific and tied to a set of services (Slack, email, etc.). While functional, this approach would increase the burden on Argo for any future integration with:

* Observability platforms that consume standardized events
* Incident management systems requiring structured event data  
* CI/CD orchestration tools that coordinate across multiple deployment systems
* Compliance and audit systems tracking deployment events
* Event-driven automation workflows

### Goals

* Enable ArgoCD to emit standardized cdEvents for continuous deployment and operations activities
* Maintain full backward compatibility with existing notification system
* Provide comprehensive event mapping covering application lifecycle, health, and operational incidents
* Enable event correlation and tracing across CI/CD pipeline stages

### Non-Goals

* Replace the existing notification system entirely
* Support cdEvents consumption (ArgoCD as event consumer)
* Add dependencies on external event brokers or message queues

## Background

### ArgoCD Notifications
ArgoCD's notification system monitors Kubernetes resources and triggers notifications based on configured conditions. It supports multiple services (Slack, email, webhooks) and uses Go templates for message formatting. The system is built on the notifications-engine.

### cdEvents Specification
cdEvents is a common specification for Continuous Delivery events, enabling interoperability between CI/CD tools. It defines:
- Standard event types for CI/CD workflows
- Consistent event structure and semantics
- CloudEvents binding for transport
- Language-specific SDKs for implementation

The specification covers multiple domains including Continuous Integration, Deployment, and Operations, with well-defined subjects and predicates for each domain.

## Proposal

We propose extending ArgoCD's notification system to support cdEvents as a new service type, allowing parallel emission of both traditional notifications and standardized cdEvents without disrupting existing workflows.

### Use cases

#### Use case 1: Multi-tool CI/CD Observability
As a platform engineer, I want to correlate deployment events from ArgoCD with build events from Tekton and test events from other tools in a unified observability dashboard that consumes standardized cdEvents.

#### Use case 2: Automated Incident Management  
As an SRE, I want ArgoCD health degradation events to automatically create incidents in my incident management system, which consumes cdEvents for standardized incident detection and correlation.

#### Use case 3: Compliance and Audit Tracking
As a compliance officer, I want to track all deployment activities across our organization using a centralized audit system that ingests standardized cdEvents from multiple deployment tools including ArgoCD.

#### Use case 4: Event-driven Automation
As a DevOps engineer, I want to trigger automated rollback procedures when ArgoCD emits service deployment failure events, enabling faster recovery without manual intervention.

### Current State Analysis

#### ArgoCD Notifications System

**Current Notification Triggers:**
- `on-created` - Application created
- `on-deleted` - Application deleted  
- `on-deployed` - Application synced and healthy
- `on-sync-succeeded` - Sync operation succeeded
- `on-sync-failed` - Sync operation failed
- `on-sync-running` - Sync operation in progress
- `on-sync-status-unknown` - Sync status unknown
- `on-health-degraded` - Application health degraded

**Trigger Conditions Examples:**
```yaml
# on-deployed trigger
- when: app.status.operationState != nil and app.status.operationState.phase in ['Succeeded'] and app.status.health.status == 'Healthy'
  description: Application is synced and healthy
  send: [app-deployed]
  oncePer: app.status.operationState?.syncResult?.revision

# on-health-degraded trigger  
- when: app.status.health.status == 'Degraded'
  description: Application health is degraded
  send: [app-health-degraded]
```

### cdEvents Specification (v0.4.1)

> **Note**: This proposal uses cdEvents v0.4.1 with event types at v0.2.0, which are currently stable and validated in a demo reference implementation. Future versions should maintain backward compatibility following semantic versioning.

**Continuous Deployment Event Types:**
- `dev.cdevents.service.deployed.0.2.0` - Service deployed to environment (new instance)
- `dev.cdevents.service.upgraded.0.2.0` - Service upgraded to new version (existing instance)
- `dev.cdevents.service.removed.0.2.0` - Service removed from environment
- `dev.cdevents.service.published.0.2.0` - Service published and accessible
- `dev.cdevents.service.rolledback.0.2.0` - Service rolled back to previous version
- `dev.cdevents.environment.created.0.2.0` - Environment created
- `dev.cdevents.environment.modified.0.2.0` - Environment modified
- `dev.cdevents.environment.deleted.0.2.0` - Environment deleted

**Continuous Operations Event Types:**
- `dev.cdevents.incident.detected.0.2.0` - Incident detected in environment
- `dev.cdevents.incident.reported.0.2.0` - Incident reported through ticketing
- `dev.cdevents.incident.resolved.0.2.0` - Incident resolved

**cdEvents Structure:**
```json
{
  "context": {
    "version": "0.4.1",
    "id": "271069a8-fc18-44f1-b38f-9d70a1695819",
    "source": "/argocd/production",
    "type": "dev.cdevents.service.deployed.0.2.0",
    "timestamp": "2025-01-20T14:27:05.315384Z"
  },
  "subject": {
    "id": "production/myService",
    "type": "service",
    "source": "/argocd/production",
    "content": {
      "environment": {
        "id": "production",
        "source": "in-cluster"
      },
      "artifactId": "pkg:git/github.com/example/myapp@abc123def"
    }
  },
  "customData": {
    "argocd": {
      "application": "myService",
      "syncStatus": "Synced",
      "healthStatus": "Healthy"
    }
  }
}
```

**CloudEvents HTTP Binding:**

cdEvents can be transmitted using CloudEvents HTTP headers alongside the JSON body:

```http
POST /webhook/cdevents HTTP/1.1
Content-Type: application/json
Ce-Specversion: 0.4.1
Ce-Type: dev.cdevents.service.deployed.0.2.0
Ce-Source: argocd/production
Ce-Id: 271069a8-fc18-44f1-b38f-9d70a1695819

{body as shown above}
```

This approach separates protocol metadata (in headers) from event payload (in body), improving compatibility with CloudEvents-aware infrastructure.

## Implementation Details/Notes/Constraints

> ⚠️ **Proposal / Big Questions**
>
> ---
>
>
> Here are the big questions – did I get the event type, subject, and data mappings correct? Do I have a good understanding of cdEvents  Am I on track?
>


### Event Mapping Strategy

#### Continuous Deployment Event Mappings

| ArgoCD Trigger | cdEvents Type | Condition | Description |
|---|---|---|---|
| `on-deployed` (first deployment) | `service.deployed.0.2.0` | New application, no previous sync | Initial deployment of service |
| `on-deployed` (upgrade) | `service.upgraded.0.2.0` | Application with previous successful sync | Service upgraded to new version |
| `on-sync-succeeded` | `service.upgraded.0.2.0` | Sync completed with new revision | Alternative upgrade detection |
| `on-deleted` | `service.removed.0.2.0` | Application being deleted | Service removal from environment |
| `on-created` | `environment.created.0.2.0` | Namespace-scoped app creation | Environment lifecycle (optional) |
| `on-deleted` | `environment.deleted.0.2.0` | Namespace-scoped app deletion | Environment lifecycle (optional) |

#### Continuous Operations Event Mappings

| ArgoCD Trigger | cdEvents Type | Condition | Description |
|---|---|---|---|
| `on-health-degraded` | `incident.detected.0.2.0` | Health transitions to Degraded | Service health incident |
| `on-sync-failed` | `incident.detected.0.2.0` | Sync operation fails | Deployment incident |
| `on-sync-status-unknown` | `incident.detected.0.2.0` | Sync status becomes unknown | Operational incident |

### Subject and Data Mapping Logic

**Service Subject Mapping:**
```yaml
# ArgoCD Application → cdEvents Service
subject:
  id: "{app.metadata.namespace}/{app.metadata.name}"
  type: "service"
  source: "/argocd/{app.metadata.namespace}"
  content:
    environment:
      id: "{app.metadata.namespace}"
      source: "{cluster-context}"
    artifactId: "pkg:git/{repo-url}@{git-commit-sha}"
```

**Environment Subject Mapping:**
```yaml
# Kubernetes Namespace → cdEvents Environment
subject:
  id: "{app.metadata.namespace}"
  type: "environment"
  source: "/argocd/{cluster-context}"
  content:
    name: "{app.metadata.namespace}"
    url: "{cluster-api-server-url}"
```

**Incident Subject Mapping:**
```yaml
# ArgoCD Health/Sync Issues → cdEvents Incident
subject:
  id: "incident-{app.metadata.namespace}-{app.metadata.name}-{timestamp}"
  type: "incident"
  source: "/argocd/{app.metadata.namespace}"
  content:
    description: "{health.status} - {sync.status}"
    environment:
      id: "{app.metadata.namespace}"
    service:
      id: "{app.metadata.namespace}/{app.metadata.name}"
    artifactId: "pkg:git/{repo-url}@{git-commit-sha}"
```

**Enhanced CustomData Examples:**

For **Service Deployed/Upgraded** events:
```json
{
  "customData": {
    "argocd": {
      "application": "webapp",
      "syncStatus": "Synced",
      "healthStatus": "Healthy",
      "targetRevision": "main",
      "cluster": "production-cluster",
      "namespace": "production"
    }
  }
}
```

For **Sync Failed (Incident)** events:
```json
{
  "customData": {
    "argocd": {
      "application": "webapp",
      "phase": "Failed",
      "syncResult": {
        "revision": "abc123",
        "resources": [...],
        "message": "one or more objects failed to apply"
      }
    }
  }
}
```



**Data Transformations:**
- **Application Identification**: `{namespace}/{name}` → Service ID
- **Git Revision**: Commit SHA → PURL format (`pkg:git/repo@sha`)
- **Environment**: Kubernetes namespace → Environment ID
- **Timestamps**: RFC3339 format as required by cdEvents
- **Source**: ArgoCD cluster context (configurable, follows `/argocd/{context}` pattern)

### CustomData Namespacing Convention

To enable interoperability in multi-tool CD ecosystems, ArgoCD-specific metadata in the `customData` field is namespaced under the `argocd` key:

```json
{
  "customData": {
    "argocd": {
      "application": "webapp",
      "syncStatus": "Synced",
      "healthStatus": "Healthy",
      "targetRevision": "main",
      "cluster": "production-cluster",
      "namespace": "production"
    }
  }
}
```

**Rationale:**
- **Tool Identification**: Clearly identifies ArgoCD-specific fields in mixed event streams
- **Collision Avoidance**: Prevents conflicts when events from multiple CD tools are aggregated
- **Query Efficiency**: Enables JSON path queries like `customData.argocd.syncStatus` in observability platforms
- **Extensibility**: Allows future correlation with other tools (e.g., `customData.github`, `customData.tekton`)

**Naming Guidelines:**
- Use the **primary tool name** as the namespace key (lowercase, e.g., `argocd`, `tekton`, `jenkins`)
- Within the namespace, use descriptive, domain-specific field names
- Avoid redundant prefixes (use `application` not `argocdApplication` within the `argocd` namespace)

**Future Considerations:**
If organizational hierarchy becomes necessary (e.g., multiple tools from the same project), the cdEvents community may recommend reverse-DNS style namespacing (e.g., `argoproj.argocd`, `argoproj.workflows`). This proposal uses single-level namespacing as sufficient for current use cases.

### Detailed examples

#### Service Deployed Event Example
```json
{
  "context": {
    "version": "0.5.0-draft",
    "id": "argocd-deployment-uuid",
    "source": "argocd/production-cluster",
    "type": "dev.cdevents.service.deployed.0.3.0-draft",
    "timestamp": "2025-01-21T14:27:05.315384Z"
  },
  "subject": {
    "id": "production/webapp",
    "type": "service",
    "content": {
      "environment": {
        "id": "production"
      },
      "artifactId": "pkg:git/github.com/example/webapp@abc123def"
    }
  }
}
```

#### Incident Detected Event Example  
```json
{
  "context": {
    "version": "0.5.0-draft", 
    "id": "argocd-incident-uuid",
    "source": "argocd/production-cluster",
    "type": "dev.cdevents.incident.detected.0.3.0-draft",
    "timestamp": "2025-01-21T14:30:12.123456Z"
  },
  "subject": {
    "id": "incident-production-webapp-20250121",
    "type": "incident",
    "content": {
      "description": "Application health degraded - Progressing",
      "environment": {"id": "production"},
      "service": {"id": "production/webapp"}
    }
  }
}
```

## Phase 1 Implementation Design

### Notification Template Architecture

The Phase 1 implementation leverages ArgoCD's existing notification engine with custom webhook templates to emit cdEvents-compliant JSON. This approach requires no modifications to ArgoCD core components.

### Template Mapping Summary

| ArgoCD Template | cdEvents Type | Trigger Condition |
|---|---|---|
| `cdevents-service-deployed` | `service.deployed` or `service.upgraded` | `on-deployed` - Application synced and healthy |
| `cdevents-service-removed` | `service.removed` | `on-deleted` - Application deleted |
| `cdevents-incident-detected` | `incident.detected` | `on-health-degraded` - Health status degraded |
| `cdevents-sync-failed` | `incident.detected` | `on-sync-failed` - Sync operation failed |
| `cdevents-incident-resolved` | `incident.resolved` | Health recovery - Degraded to Healthy |

#### Template Examples

##### Service Deployed/Upgraded Template
```yaml
## argo-notifications yaml

  # Template for service deployment events
  template.cdevents-service-deployed: |
    webhook:
      cdevents:
        method: POST
        body: |
          {
            "context": {
            },
            "subject": {
            },
            "customData": {
            }
          }

  # Trigger binding
  trigger.on-deployed-cdevents: |
    - when: app.status.operationState != nil asnd app.status.operationState.phase in ['Succeeded'] and app.status.health.status == 'Healthy'
      send: [cdevents-service-deployed]
      oncePer: app.status.operationState?.syncResult?.revision
```

### Security Considerations

* **Authentication**: Support for bearer tokens and basic auth for webhook endpoints
* **TLS**: Webhook service supports TLS verification (can be disabled for development)
* **Secret Management**: Sensitive tokens stored in Kubernetes secrets
* **Rate Limiting**: Consider implementing rate limits on the webhook receiver
* **Event Signing**: Future enhancement to add event signature verification

## Transport Mechanism Recommendation

### Phase 1: Simple HTTP Webhook Receiver

For initial prototype and testing, we recommend a simple HTTP webhook receiver that can...

- *Is there an existing example/demo service from cdEvents group?*
- Yes - see cdViz - https://github.com/cdviz-dev/cdviz/

## Gaps and Future Considerations

### Unmapped ArgoCD Events

The following ArgoCD events do not have direct cdEvents equivalents but could be valuable for the specification:

| ArgoCD Event | Potential cdEvents Type | Rationale |
|---|---|---|
| `on-created` | `service.configured` | Application configuration created |
| `on-sync-running` | `deployment.started` | Deployment process initiated |
| Environment events | `environment.*` | Namespace lifecycle management |
| Multi-app sync | `deploymentset.*` | Batch deployment operations |
| Progressive sync | `deployment.progressing` | Canary/blue-green deployments |

### Future Enhancement Opportunities

#### Phase 2: Native Integration
- Direct integration using cdEvents Go SDK
- Eliminate webhook overhead
- Built-in schema validation
- Event linking and correlation support

#### Phase 3: Advanced Features
- Bidirectional event flow (ArgoCD consuming cdEvents)
- Event-driven GitOps workflows
- Cross-cluster event federation
- Advanced filtering and routing

### ArgoCD-Specific Metadata Preservation

The `customData` field in cdEvents allows preservation of ArgoCD-specific information:
- Application labels and annotations
- Sync policy details
- Health check configuration
- Multi-source application data
- Application of Applications patterns

### Risks and Mitigations

#### Technical Risks
- **Event Volume**: High-frequency syncs could generate excessive events
  - *Mitigation*: Implement rate limiting and event deduplication
- **Delivery Failures**: Webhook endpoint unavailability
  - *Mitigation*: Implement retry logic with exponential backoff
- **Schema Evolution**: cdEvents spec changes
  - *Mitigation*: Version templates and maintain compatibility matrix

#### Operational Risks  
- **Configuration Complexity**: Additional templates and triggers to manage
  - *Mitigation*: Provide pre-built template library and documentation
- **Monitoring Overhead**: New event pipeline to monitor
  - *Mitigation*: Include metrics and health endpoints in receiver

### Upgrade / Downgrade Strategy

- **Upgrade**: New templates can be added without affecting existing notifications
- **Downgrade**: Remove cdEvents templates and webhook configuration
- **Compatibility**: Maintain parallel traditional and cdEvents notifications during transition

## Drawbacks

- Additional configuration complexity for operators
- Potential performance impact from webhook calls
- Dependency on external webhook receiver
- Learning curve for cdEvents specification

## Alternatives

### Alternative 1: Direct Notifications Engine Modification
Modify the notifications-engine to natively support cdEvents format
- **Pros**: Better performance, no webhook overhead
- **Cons**: Requires modifying upstream project



## Appendices

### Appendix A: Sequence Diagrams
*Placeholder for event flow diagrams*

### Appendix B: Architecture Diagrams  
*Placeholder for component interaction diagrams*

### Appendix C: cdEvents SDK Reference
- [Go SDK](https://github.com/cdevents/sdk-go)
- [Specification](https://cdevents.dev/docs/)
- [CloudEvents Binding](https://github.com/cdevents/spec/blob/main/cloudevents-binding.md) 