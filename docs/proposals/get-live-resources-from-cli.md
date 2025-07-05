---
title: Get Live Resources from CLI
authors:
  - "@cjcocokrisp" 
sponsors:
  - TBD        
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2025-06-17
last-updated: 2025-07-03
---

# Get Live Resources from the ArgoCD CLI

Add the ability to get the live manifest of a resource from the ArgoCD CLI.

## Open Questions [optional]

- Command specific naming convention. (See Implementation Details for what I'm thinking)

## Summary

In GitHub issue [#22945](https://github.com/argoproj/argo-cd/issues/22945), the user mentions that there
is no way to get the IPs of Pods directly using the ArgoCD CLI. This information is valuable in debugging 
and monitoring tasks. Having this feature would require the live manifests of resources within an 
application to be viewable through the CLI. 

The ArgoCD API already has a [swagger endpoint](https://cd.apps.argoproj.io/swagger-ui#tag/ApplicationService/operation/ApplicationService_GetResource)
for this operation and live manifests are viewable in the Web UI. It would be nice to have this 
functionality wrapped into the CLI as well. 

There also already exists CLI commands for deleting and patching a resource. I could not find 
a specific reason for a get command not existing either.

In initial discussion in the above issues it was decided that there should be an
application pods command. However it was decided that just a command for live
manifests of a resource is needed.

This proposal will describe the outline for these commands.

## Motivation

The motivation of this feature is to allow for better support of getting details 
of live manifests in the ArgoCD CLI. This will help with monitoring and debugging 
tasks. It also will reduce the need for direct kubectl access in environments where
access is restricted.

If you were to try and do this without this command here is how you would get 
the IP of a Pod in an application.

1. Get the resources of the application using `argocd app resources`
2. If the Pod is listed you would then need to use `kubectl describe Pod <name of the pod>` 
or use `kubectl get pods | grep `

As mentioned if there was no access to kubectl then this task would be impossible.

### Goals

- Improve observability offered by ArgoCD CLI. 
  - Allow for the live manifest of a resource to be obtained. 
  - View the IPs of Pods. 

### Non-Goals

- Having specific commands for getting the live manifest of every resource in
an application.

## Proposal

### Use cases

#### Use case 1:
As a user, I would like to easily view the IPs of the Pods in my application. 

#### Use case 2:
As a user, I would like to access the live manifests of a resource in a application
owned by ArgoCD but do not have direct kubectl access. 

### Implementation Details/Notes/Constraints [optional]

What are the caveats to the implementation? What are some important details that didn't come across
above. Go in to as much detail as necessary here. This might be a good place to talk about core
concepts and how they relate.

You may have a work-in-progress Pull Request to demonstrate the functioning of the enhancement you are proposing.

#### Get Resource Command Syntax

Contributed by @cjcocokrisp

```argocd app get-resource APPNAME```

#### Example Usage
To  Get a Pod
```argocd app get-resource APPNAME --kind Pod --resourcename nginx-XXXXXXXXXXXXXX```
To Get Something Else
```argocd app get-resource APPNAME -k Service -r nginx-svc```

#### Flags

```
Flag | Type | Description
    --resource-name | string | The name of the resource [REQUIRED]
    --kind | string | The kind of the resource [REQUIRED]
    --app-namespace | string | The namespace of the parent app if none is provided will default to `argocd` namespace
-o, --output | string | yaml or json, will default to yaml
    --project | string | The project of the resource, if none is provided will default to being empty         
    --show-managed-fields | bool | Whether or not to show managed fields, will default to false to match UI behavior
    --filter-fields | []string | a comma seperated list of fields to filter example for getting pod IPs would be status.podIP
```

#### Output

Would output the live manifest of a resource in an application in YAML, JSON or table (wide). 

### Security Considerations

- If credentials for Argo CD CLI login are stolen, specific information about the live state of resources 
could be leaked. 

### Risks and Mitigations

A risk of this feature is that it would require updating if the API endpoint it relies on is ever
modified or removed this command will probably need to be updated as well. 

### Upgrade / Downgrade Strategy

- ArgoCD CLI would just need to be updated. 

## Drawbacks

Might not be that needed in the CLI. 

## Alternatives

Users would need this feature could create their own scripts that use the `resources` command
to get all resources and then calls the API endpoint. Or they could just check the Web UI for 
the live manifests. 