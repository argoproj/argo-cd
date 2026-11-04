# Manifest compression dictionary

`dictionary.jsonl` is the sample used to build the zstd dictionaries for the `zstd` manifest
compression type in the cluster cache (`resource.manifest.compression: zstd`).

Most cached manifests are small, and a compressor working on one small object at a time has
little to match against. A dictionary gives zstd the strings that Kubernetes objects have in
common (field names, label and annotation keys, condition reasons, `managedFields` structure)
before it sees the first byte. Large objects such as CRDs already compress well on their own, so
the sample is aimed at the common kinds that large clusters have many copies of.

## How the dictionaries are built

At first use of the `zstd` compression type, the cache:

1. parses each line of `dictionary.jsonl`;
2. serializes it with the cache's own serializers, once as JSON (also used for `jsoniter`, which
   produces the same bytes) and once as msgpack with sorted map keys;
3. concatenates the results and uses each as a zstd raw-content dictionary.

Because the cache's serializers produce the dictionary bytes, the dictionary always matches the
byte format of the stored manifests. The result is identical on every run.

Compressed manifests are only kept in memory and are never shared between processes, so changing
this file is safe in any release. A change only affects compression ratios.

## Rules for the sample

- **Built-in kinds only.** A dictionary entry for a CRD-based kind only helps clusters that run
  that project. Stick to the core Kubernetes API.
- **No per-object values.** Use an empty string (`""`) for names, namespaces, UIDs, timestamps,
  images and similar. `"name":""` still gives zstd the reusable `"name":"` prefix.
- **Keep values that are the same everywhere.** Defaults such as `"imagePullPolicy":"IfNotPresent"`,
  condition types, reasons and messages written by built-in controllers, and well-known label and
  annotation keys all belong here.
- **Don't repeat invariant content.** Shared metadata, `managedFields` structure and conditions
  appear once. Each kind's line carries only what differs: its `apiVersion`, `kind`, and the spec
  and status fields specific to it.
- **Most common kinds last.** zstd references the end of a dictionary most cheaply, so the most
  common kinds (Deployment, Service, ConfigMap, Secret, ServiceAccount) come last.
- **Stay under 64 KiB** for each serialized format. `TestManifestDictionary` checks this.

## Measuring a change

Compression results depend on the objects being compressed, so check a change against manifests
from a real cluster, not against this sample. Compare `zstd` against `gzip-bestspeed` on the
stored size of manifests that Argo CD caches: tracked top-level resources, excluding CRDs. When
the sample was created, `zstd` with this dictionary stored those manifests in roughly 43–52% less
memory than `gzip-bestspeed` did before `trimExcessCapacity` was added, compared with 24–28% for
`zstd` without a dictionary.
