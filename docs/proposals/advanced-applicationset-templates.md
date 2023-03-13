---
title: Advanced ApplicationSet Templates
authors:
  - "@crenshaw-dev"
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2023-03-13
last-updated: 2023-03-13
---

# Advanced ApplicationSet Templates


## Summary

The proposal is to make an ApplicationSet's `template` field a raw JSON field. That field is currently strictly typed, 
having the exact same structure as the Application CRD. Making the field raw JSON will permit more flexible templating,
unblocking much more advanced ApplicationSet use cases.

## Motivation

Use cases enabled:

* Enabling/disabling auto-sync based on a parameter.
* Using more than one config management tool in the Application `sources` field (e.g. `helm` and `kustomize` in the same ApplicationSet).
* Templating non-string types, such as full objects, arrays, booleans, and numbers.

### Goals

1. Enable the first to use cases listed above: toggling auto-sync and using heterogeneous sources.
2. Pave the way for the last goal (templating arbitrary field types), without necessarily implementing it.

### Non-Goals

1. Providing full text templating. The user will still be constrained to writing templates within string fields and keys.

## Proposal

### Use cases

#### Use case 1: heterogeneous source types

As a user, I would like to create an ApplicationSet which deploys Applications with more than one type of source 
([issue](https://github.com/argoproj/argo-cd/issues/9177)).

Today, if I specify more than one source type, the ApplicationSet produces an invalid application. 

For example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  goTemplate: true
  generators:
  - list:
      elements:
      - cluster: engineering-dev
        url: https://kubernetes.default.svc
        sourceType: helm
      - cluster: engineering-prod
        url: https://kubernetes.default.svc
        sourceType: kustomize
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: "{{.sourceType}}"
        helm:
          parameters:
            - some-param
        kustomize:
          images:
            - some=image
      destination:
        server: '{{.url}}'
        namespace: guestbook
```

There's currently no way to toggle one source type off while leaving the other enabled. This proposal would enable this
syntax:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  goTemplate: true
  generators:
  - list:
      elements:
      - cluster: engineering-dev
        url: https://kubernetes.default.svc
        sourceType: helm
      - cluster: engineering-prod
        url: https://kubernetes.default.svc
        sourceType: kustomize
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: "{{.sourceType}}"
        '{{.sourceType == "helm" | ternary "helm" "noHelm" }}':
          parameters:
            - some-param
        '{{.sourceType == "kustomize" | ternary "kustomize" "noKustomize" }}':
          images:
            - some=image
      destination:
        server: '{{.url}}'
        namespace: guestbook
```

#### Use case 2: toggling auto-sync

As a user, I would like to use an ApplicationSet parameter to toggle whether auto-sync is enabled.

The current problem is that the value of `syncPolicy.automated` is an object. Today's templating only allows templating
string fields.

This proposal would enable this syntax:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  goTemplate: true
  generators:
  # ...
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: "{{.sourceType}}"
      destination:
        server: '{{.url}}'
        namespace: guestbook
      syncPolicy:
        '{{ ternary "automated" "noAuto" .automated }}': {}
```

#### Use case 3: templating arbitrary field types

As a user, I would like to be able to set field values besides strings. For example, I should be able to use a parameter
as a boolean.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  goTemplate: true
  generators:
  # ...
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: "{{.sourceType}}"
      destination:
        server: '{{.url}}'
        namespace: guestbook
      syncPolicy:
        automated:
          prune: '{{.prune}}'
```

This feature will _not_ be in the initial implementation. But since `template` will be made raw JSON, the above will be
valid YAML. We can later examine unmarshaling trickery to support non-string field templating.

### Implementation Details/Notes/Constraints

Here is the PR for an initial implementation: https://github.com/argoproj/argo-cd/pull/11567

### Detailed examples

The examples in the use case should suffice. If not, I will update this section.

### Security Considerations

ApplicationSets are currently admin-only resources. It is not safe to let non-admins create or update ApplicationSets.

However, ApplicationSet generator inputs may _not_ be admin-controlled.

By making the whole `template` field raw JSON, we offer ApplicationSet authors new ways to open security holes.

For example, consider this ApplicationSet:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  goTemplate: true
  generators:
  # ...
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        '{{.pathKey}}': "{{.path}}"
      destination:
        server: '{{.url}}'
        namespace: guestbook
```

The expectation might be that the user sets `.pathKey` to `"path"` if they want to set a path and `.pathKey` to 
`"noPath"` if they want to leave the `path` field blank.

But since `.pathKey` may be an arbitrary value, the user is now empowered to set `.pathKey` to `"repoURL"` and
`.path` to some other repo.

I consider this a relatively low-risk problem. The template above is easy to read, and it is easy to understand the 
implications of making a field name completely dynamic. But we should document this risk.

### Risks and Mitigations

#### Risk 1: the Kubernetes API will not catch typos in the `template` field

Since the `template` field is plain JSON, the Kubernetes API will not catch typos in the field name. For example, if I
set `porject` instead of `project`, the Kubernetes API will happily accept that manifest. I will not get the error until
the ApplicationSet controller tries to create the invalid Application.

#### Risk 2: ApplicationSets using advanced templating will be less readable

The current templating syntax is very simple. It is easy to read and understand. The new templating syntax is more
powerful, but it is also more complex. It is harder to read and understand.

I think this is a reasonable tradeoff. But it does make building some kind of diffing tool more important. Users should
have a way to test the output of a hypothetical ApplicationSet before they actually create it.

#### Risk 3: no type-checking for the `template` field

The ApplicationSet controller sometimes needs to refer to fields in the `template` field. We can currently access those
fields using Go types. For example, we can access the `spec.source.repoURL` field using `template.Spec.Source.RepoURL`.

If we make the `template` field raw JSON, we lose the ability to access fields using Go types. We will need to use
string-based field accessors.

The lack of typing makes it more difficult to write code and to detect bugs while we code.

Thankfully, the places where we currently reach into the `template` field are relatively few and simple.

### Upgrade / Downgrade Strategy

This is a backwards-compatible change. Existing ApplicationSets will continue to work as they do today.

If a user starts using the new templating syntax for field names, downgrading the CRD will cause them to lose the 
go-templated field names. We should either 1) bump the ApplicationSet CRD version or 2) warn the users not to adopt new
field name templating until they are confident that they will not downgrade.

## Drawbacks

1. Loss of CRD validation for the `template` field.

## Alternatives

We could just add a `stringTemplate` field and let folks write arbitrary go templates. Personally, I think the relative
complexity of a fully-templated spec would make ApplicationSets unnecessarily complicated. Most users need the ability
to toggle one or two fields. This change allows that.
