# Go Template

## Introduction

ApplicationSet is able to use [Go Text Template](https://pkg.go.dev/text/template). To activate this feature, add 
`goTemplate: true` to your ApplicationSet manifest.

The [Sprig function library](https://masterminds.github.io/sprig/) (except for `env`, `expandenv` and `getHostByName`) 
is available in addition to the default Go Text Template functions.

An additional `normalize` function makes any string parameter usable as a valid DNS name by replacing invalid characters 
with hyphens and truncating at 253 characters. This is useful when making parameters safe for things like Application
names.

## Motivation

Go Template is the Go Standard for string templating. It is also more powerful than fasttemplate (the default templating 
engine) as it allows doing complex templating logic.

## Limitations

Go templates are applied on a per-field basis, and only on string fields. Here are some examples of what is **not** 
possible with Go text templates:

- Templating a boolean field.

        ::yaml
        apiVersion: argoproj.io/v1alpha1
        kind: ApplicationSet
        spec:
          goTemplate: true
          template:
            spec:
              source:
                helm:
                  useCredentials: "{{.useCredentials}}"  # This field may NOT be templated, because it is a boolean field.

- Templating an object field:

        ::yaml
        apiVersion: argoproj.io/v1alpha1
        kind: ApplicationSet
        spec:
          goTemplate: true
          template:
            spec:
              syncPolicy: "{{.syncPolicy}}"  # This field may NOT be templated, because it is an object field.

- Using control keywords across fields:

        ::yaml
        apiVersion: argoproj.io/v1alpha1
        kind: ApplicationSet
        spec:
          goTemplate: true
          template:
            spec:
              source:
                helm:
                  parameters:
                  # Each of these fields is evaluated as an independent template, so the first one will fail with an error.
                  - name: "{{range .parameters}}"
                  - name: "{{.name}}"
                    value: "{{.value}}"
                  - name: throw-away
                    value: "{{end}}"


## Migration guide

### Globals

All your templates must replace parameters with GoTemplate Syntax:

Example: `{{ some.value }}` becomes `{{ .some.value }}`

### Cluster Generators

By activating Go Templating, `{{ .metadata }}` becomes an object.

- `{{ metadata.labels.my-label }}` becomes `{{ index .metadata.labels "my-label" }}`
- `{{ metadata.annotations.my/annotation }}` becomes `{{ index .metadata.annotations "my/annotation" }}`

### Git Generators

By activating Go Templating, `{{ .path }}` becomes an object. Therefore, some changes must be made to the Git 
generators' templating:

- `{{ path }}` becomes `{{ .path.path }}`
- `{{ path[n] }}` becomes `{{ index .path.segments n }}`

Here is an example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-addons
spec:
  generators:
  - git:
      repoURL: https://github.com/argoproj/argo-cd.git
      revision: HEAD
      directories:
      - path: applicationset/examples/git-generator-directory/cluster-addons/*
  template:
    metadata:
      name: '{{path.basename}}'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: '{{path}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: '{{path.basename}}'
```

becomes

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-addons
spec:
  goTemplate: true
  generators:
  - git:
      repoURL: https://github.com/argoproj/argo-cd.git
      revision: HEAD
      directories:
      - path: applicationset/examples/git-generator-directory/cluster-addons/*
  template:
    metadata:
      name: '{{.path.basename}}'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: '{{.path.path}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: '{{.path.basename}}'
```

It is also possible to use Sprig functions to construct the path variables manually:

| with `goTemplate: false` | with `goTemplate: true` | with `goTemplate: true` + Sprig |
| ------------ | ----------- | --------------------- |
| `{{path}}` | `{{.path.path}}` | `{{.path.path}}` |
| `{{path.basename}}` | `{{.path.basename}}` | `{{base .path.path}}` |
| `{{path.filename}}` | `{{.path.filename}}` | `{{.path.filename}}` |
| `{{path.basenameNormalized}}` | `{{.path.basenameNormalized}}` | `{{normalize .path.path}}` |
| `{{path.filenameNormalized}}` | `{{.path.filenameNormalized}}` | `{{normalize .path.filename}}` |
| `{{path[N]}}` | `-` | `{{index .path.segments N}}` |

## Examples

### Basic Go template usage

This example shows basic string parameter substitution.

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
        url: https://1.2.3.4
      - cluster: engineering-prod
        url: https://2.4.6.8
      - cluster: finance-preprod
        url: https://9.8.7.6
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
    spec:
      project: my-project
      source:
        repoURL: https://github.com/infra-team/cluster-deployments.git
        targetRevision: HEAD
        path: guestbook/{{.cluster}}
      destination:
        server: '{{.url}}'
        namespace: guestbook
```

### Fallbacks for unset parameters

For some generators, a parameter of a certain name might not always be populated (for example, with the values generator
or the git files generator). In these cases, you can use a Go template to provide a fallback value.

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
      - cluster: engineering-prod
        url: https://kubernetes.default.svc
        nameSuffix: -my-name-suffix
  template:
    metadata:
      name: '{{.cluster}}{{default "" .nameSuffix}}'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: applicationset/examples/list-generator/guestbook/{{.cluster}}
      destination:
        server: '{{.url}}'
        namespace: guestbook
```

This ApplicationSet will produce an Application called `engineering-dev` and another called 
`engineering-prod-my-name-suffix`.
