# Merge Generator

The Merge generator combines parameters produced by the base (first) generator with matching parameter sets produced by subsequent generators. A _matching_ parameter set has the same values for the configured _merge keys_. _Non-matching_ parameter sets are discarded. Override precedence is bottom-to-top: the values from a matching parameter set produced by generator 3 will take precedence over the values from the corresponding parameter set produced by generator 2.

Using a Merge generator is appropriate when a subset of parameter sets require overriding.

## Example: Base Cluster generator + override Cluster generator + List generator 

As an example, imagine that we have two clusters:

- A `staging` cluster (at `https://1.2.3.4`)
- A `production` cluster (at `https://2.4.6.8`)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-git
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
    # merge 'parent' generator
    - merge:
        mergeKeys:
          - server
        generators:
          - clusters:
              values:
                kafka: 'true'
                redis: 'false'
          # For clusters with a specific label, enable Kafka.
          - clusters:
              selector:
                matchLabels:
                  use-kafka: 'false'
              values:
                kafka: 'false'
          # For a specific cluster, enable Redis.
          - list:
              elements: 
                - server: https://2.4.6.8
                  values.redis: 'true'
  template:
    metadata:
      name: '{{.name}}'
    spec:
      project: '{{index .metadata.labels "environment"}}'
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: app
        helm:
          parameters:
            - name: kafka
              value: '{{.values.kafka}}'
            - name: redis
              value: '{{.values.redis}}'
      destination:
        server: '{{.server}}'
        namespace: default
```

The base Cluster generator scans the [set of clusters defined in Argo CD](Generators-Cluster.md), finds the staging and production cluster secrets, and produces two corresponding sets of parameters:
```yaml
- name: staging
  server: https://1.2.3.4
  values.kafka: 'true'
  values.redis: 'false'
  
- name: production
  server: https://2.4.6.8
  values.kafka: 'true'
  values.redis: 'false'
```

The override Cluster generator scans the [set of clusters defined in Argo CD](Generators-Cluster.md), finds the staging cluster secret (which has the required label), and produces the following parameters:
```yaml
- name: staging
  server: https://1.2.3.4
  values.kafka: 'false'
```

When merged with the base generator's parameters, the `values.kafka` value for the staging cluster is set to `'false'`.
```yaml
- name: staging
  server: https://1.2.3.4
  values.kafka: 'false'
  values.redis: 'false'

- name: production
  server: https://2.4.6.8
  values.kafka: 'true'
  values.redis: 'false'
```

Finally, the List cluster generates a single set of parameters:
```yaml
- server: https://2.4.6.8
  values.redis: 'true'
```

When merged with the updated base parameters, the `values.redis` value for the production cluster is set to `'true'`. This is the merge generator's final output:
```yaml
- name: staging
  server: https://1.2.3.4
  values.kafka: 'false'
  values.redis: 'false'

- name: production
  server: https://2.4.6.8
  values.kafka: 'true'
  values.redis: 'true'
```

## Example: Use value interpolation in merge

Some generators support additional values and interpolating from generated variables to selected values. This can be used to teach the merge generator which generated variables to use to combine different generators.

The following example combines discovered clusters and a git repository by cluster labels and the branch name:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-git
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
    # merge 'parent' generator:
    # Use the selector set by both child generators to combine them.
    - merge:
        mergeKeys:
          # Note that this would not work with goTemplate enabled,
          # nested merge keys are not supported there.
          - values.selector
        generators:
          # Assuming, all configured clusters have a label for their location:
          # Set the selector to this location.
          - clusters:
              values:
                selector: '{{index .metadata.labels "location"}}'
          # The git repo may have different directories which correspond to the
          # cluster locations, using these as a selector.
          - git:
              repoURL: https://github.com/argoproj/argocd-example-apps/
              revision: HEAD
              directories:
              - path: '*'
              values:
                selector: '{{.path.path}}'
  template:
    metadata:
      name: '{{.name}}'
    spec:
      project: '{{index .metadata.labels "environment"}}'
      source:
        repoURL: https://github.com/argoproj/argocd-example-apps/
        # The cluster values field for each generator will be substituted here:
        targetRevision: HEAD
        path: '{{.path.path}}'
      destination:
        server: '{{.server}}'
        namespace: default
```

Assuming a cluster named `germany01` with the label `metadata.labels.location=Germany` and a git repository containing a directory called `Germany`, this could combine to values as follows:

```yaml
  # From the cluster generator
- name: germany01
  server: https://1.2.3.4
  # From the git generator
  path: Germany
  # Combining selector with the merge generator
  values.selector: 'Germany'
  # More values from cluster & git generator
  # […]
```


## Restrictions

1. You should specify only a single generator per array entry. This is not valid:

        - merge:
            generators:
            - list: # (...)
              git: # (...)

    - While this *will* be accepted by Kubernetes API validation, the controller will report an error on generation. Each generator should be specified in a separate array element, as in the examples above.

1. The Merge generator does not support [`template` overrides](Template.md#generator-templates) specified on child generators. This `template` will not be processed:

        - merge:
            generators:
              - list:
                  elements:
                    - # (...)
                  template: { } # Not processed

1. Combination-type generators (Matrix or Merge) can only be nested once. For example, this will not work:

        - merge:
            generators:
              - merge:
                  generators:
                    - merge:  # This third level is invalid.
                        generators:
                          - list:
                              elements:
                                - # (...)

1. Merging on nested values while using `goTemplate: true` is currently not supported, this will not work

        spec:
          goTemplate: true
          generators:
          - merge:
              mergeKeys:
                - values.merge

1. When using a Merge generator nested inside another Matrix or Merge generator, [Post Selectors](Generators-Post-Selector.md) for this nested generator's generators will only be applied when enabled via `spec.applyNestedSelectors`.

        - merge:
            generators:
              - merge:
                  generators:
                    - list
                        elements:
                          - # (...)
                      selector: { } # Only applied when applyNestedSelectors is true
