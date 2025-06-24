---
title: ArgoCD to cdEvents Integration
authors:
  - lukepatrick
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2025-01-21
last-updated: 2025-01-21
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

## Summary

ArgoCD currently provides a notification system that can send alerts to various destinations (Slack, email, webhooks) based on application lifecycle events. However, these notifications are "argo specific" and may not easily integrate with broader CI/CD toolchain ecosystems.

This proposal extends ArgoCD's existing notification controller to emit cdEvents-compliant events alongside traditional notifications. cdEvents is an emerging specification for standardizing events across the software development lifecycle, enabling better interoperability between tools.

The integration will allow ArgoCD to participate in standardized CI/CD event ecosystems while maintaining backward compatibility with existing notification workflows.

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

**Architecture:**
- **Notification Controller** (`notification_controller/`) - Kubernetes controller that watches ArgoCD Application CRDs and triggers notifications based on state changes
- **Notifications Catalog** (`notifications_catalog/`) - Pre-defined notification templates and triggers for common scenarios
- **Engine Integration** - Uses `github.com/argoproj/notifications-engine` for delivery to various services (Slack, email, webhooks, etc.)

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

### cdEvents Specification (v0.5.0-draft)

**Continuous Deployment Event Types:**
- `dev.cdevents.service.deployed.0.3.0-draft` - Service deployed to environment
- `dev.cdevents.service.upgraded.0.3.0-draft` - Service upgraded to new version
- `dev.cdevents.service.removed.0.3.0-draft` - Service removed from environment
- `dev.cdevents.service.published.0.3.0-draft` - Service published and accessible
- `dev.cdevents.service.rolledback.0.3.0-draft` - Service rolled back to previous version
- `dev.cdevents.environment.created.0.3.0-draft` - Environment created
- `dev.cdevents.environment.modified.0.3.0-draft` - Environment modified
- `dev.cdevents.environment.deleted.0.3.0-draft` - Environment deleted

**Continuous Operations Event Types:**
- `dev.cdevents.incident.detected.0.3.0-draft` - Incident detected in environment
- `dev.cdevents.incident.reported.0.3.0-draft` - Incident reported through ticketing
- `dev.cdevents.incident.resolved.0.3.0-draft` - Incident resolved
- `dev.cdevents.ticket.created.0.2.0-draft` - Ticket created
- `dev.cdevents.ticket.updated.0.2.0-draft` - Ticket updated
- `dev.cdevents.ticket.closed.0.2.0-draft` - Ticket closed

**cdEvents Structure:**
```json
{
  "context": {
    "version": "0.5.0-draft",
    "id": "271069a8-fc18-44f1-b38f-9d70a1695819",
    "source": "/event/source/123",
    "type": "dev.cdevents.service.deployed.0.3.0-draft",
    "timestamp": "2023-03-20T14:27:05.315384Z"
  },
  "subject": {
    "id": "myService123",
    "type": "service",
    "content": {
      "environment": {"id": "production"},
      "artifactId": "pkg:oci/myapp@sha256%3A..."
    }
  }
}
```

## Implementation Details/Notes/Constraints

> ⚠️ **Proposal / Big Questions**
>
> ---
>
>
> Here are the big questions – did I get the event type, subject, and data mappings correct? Do I have a good understanding of cdEvents  Am I on track?
>


### Event Mapping Strategy

#### Continuous Deployment Mappings

| ArgoCD Trigger | cdEvents Type | Condition | Description |
|---|---|---|---|
| `on-deployed` (first deployment) | `service.deployed` | New application, no previous sync | Initial deployment of service |
| `on-deployed` (upgrade) | `service.upgraded` | Application with previous successful sync | Service upgraded to new version |
| `on-sync-succeeded` | `service.upgraded` | Sync completed with new revision | Alternative upgrade detection |
| `on-deleted` | `service.removed` | Application being deleted | Service removal from environment |
| `on-created` | `environment.created` | Namespace-scoped app creation | Environment lifecycle (optional) |
| `on-deleted` | `environment.deleted` | Namespace-scoped app deletion | Environment lifecycle (optional) |
| Service accessibility | `service.published` | Service becomes externally accessible | Service ready for consumption |

#### Continuous Operations Mappings

| ArgoCD Trigger | cdEvents Type | Condition | Description |
|---|---|---|---|
| `on-health-degraded` | `incident.detected` | Health transitions to Degraded | Service health incident |
| `on-sync-failed` | `incident.detected` | Sync operation fails | Deployment incident |
| `on-sync-status-unknown` | `incident.detected` | Sync status becomes unknown | Operational incident |
| Health recovery | `incident.resolved` | Health transitions from Degraded to Healthy | Health incident resolution |
| Sync recovery | `incident.resolved` | Sync succeeds after failure | Deployment incident resolution |

### Subject and Data Mapping Logic

**Service Subject Mapping:**
```yaml
# ArgoCD Application → cdEvents Service
subject:
  id: "{app.metadata.namespace}/{app.metadata.name}"
  type: "service"
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
  content:
    description: "{health.status} - {sync.status}"
    environment:
      id: "{app.metadata.namespace}"
    service:
      id: "{app.metadata.namespace}/{app.metadata.name}"
    artifactId: "pkg:git/{repo-url}@{git-commit-sha}"
```

**Data Transformations:**
- **Application Identification**: `{namespace}/{name}` → Service ID
- **Git Revision**: Commit SHA → PURL format (`pkg:git/repo@sha`)
- **Environment**: Kubernetes namespace → Environment ID
- **Timestamps**: RFC3339 format as required by cdEvents
- **Source**: ArgoCD cluster context (configurable)

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

### Security Considerations

* There are probably some

### Risks and Mitigations

#### Technical Risks
- Performance, Event Volume, Delivery Failure

#### Operational Risks  
- More Configuration / message bus integration

### Upgrade / Downgrade Strategy


## Drawbacks



## Alternatives

* Notifications Engine
* 