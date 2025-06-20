---
title: Subscribe to a notification from the Application Details page
authors:
  - "@aborilov"
sponsors:
  - TBD
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: 2022-08-16
last-updated: 2022-08-16
---

# Subscribe to a notification from the Application Details page

Provide the ability to subscribe to a notification from the Application Details page

## Summary

Allow users to subscribe application with a notification from the Application Details page
using provided instruments for selecting available triggers and services.

## Motivation

It is already possible to subscribe to notifications by modifying annotations however this is a pretty
poor user experience. Users have to understand annotation structure and have to find available services and triggers in configmap. 

### Goals

Be able to subscribe to a notification from the Application Details page without forcing users to read the notification configmap.

### Non-Goals

We provide only ability to select existing services and triggers, we don't provide instruments to add/edit/delete notification services and triggers.

## Proposal

Two changes are required:

* Implement notifications API that would expose a list of configured triggers and services
* Implement UI that leverages notifications API and helps users to create a correct annotation.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1:
As a user, I would like to be able to subscribe application to a notification from the Application Details Page
without reading knowing of annotation format and reading notification configmap.

### Implementation Details/Notes/Constraints [optional]

Three read-only API endpoints will be added to provide a list of notification services, triggers, and templates.

```
message Triggers { repeated string triggers = 1; }
message TriggersListRequest {}
message Services { repeated string services = 1; }
message ServicesListRequest {}
message Templates { repeated string templates = 1; }
message TemplatesListRequest {}
service NotificationService {
	rpc ListTriggers(TriggersListRequest) returns (Triggers) {
		option (google.api.http).get = "/api/v1/notifications/triggers";
	}
	rpc ListServices(ServicesListRequest) returns (Services) {
		option (google.api.http).get = "/api/v1/notifications/services";
	}
	rpc ListTemplates(TemplatesListRequest) returns (Templates) {
		option (google.api.http).get = "/api/v1/notifications/templates";
	}
}
```

### Detailed examples

### Security Considerations

New API endpoints are available only for authenticated users.  API endpoints response does not contain any sensitive data.

### Risks and Mitigations

TBD

### Upgrade / Downgrade Strategy

By default, we don't have a notification configmap in the system; in that case, API should return an empty list instead of erroring.

## Drawbacks


## Alternatives

Continue to do that manually.

