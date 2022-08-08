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
      name: '{{name}}'
    spec:
      project: '{{metadata.labels.environment}}'
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: app
        helm:
          parameters:
            - name: kafka
              value: '{{values.kafka}}'
            - name: redis
              value: '{{values.redis}}'
      destination:
        server: '{{server}}'
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

## Restrictions

1. You should specify only a single generator per array entry. This is not valid:
```yaml
- merge:
    generators:
     - list: # (...)
       git: # (...)
```
    - While this *will* be accepted by Kubernetes API validation, the controller will report an error on generation. Each generator should be specified in a separate array element, as in the examples above.
1. The Merge generator does not support [`template` overrides](Template.md#generator-templates) specified on child generators. This `template` will not be processed:
```yaml
- merge:
    generators:
      - list:
          elements:
            - # (...)
          template: { } # Not processed
```
1. Combination-type generators (Matrix or Merge) can only be nested once. For example, this will not work:
```yaml
- merge:
    generators:
      - merge:
          generators:
            - merge:  # This third level is invalid.
                generators:
                  - list:
                      elements:
                        - # (...)
```
