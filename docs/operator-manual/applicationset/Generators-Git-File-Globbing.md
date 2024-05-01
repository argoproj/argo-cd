# Git File Generator Globbing

## Problem Statement

The original and default implementation of the Git file generator does very greedy globbing. This can trigger errors or catch users off-guard. For example, consider the following repository layout:

```
└── cluster-charts/
    ├── cluster1
    │   ├── mychart/
    │   │   ├── charts/
    │   │   │    └── mysubchart/
    │   │   │        ├── values.yaml
    │   │   │        └── etc…
    │   │   ├── values.yaml
    │   │   └── etc…
    │   └── myotherchart/
    │       ├── values.yaml
    │       └── etc…
    └── cluster2
        └── etc…
```

In `cluster1` we have two charts, one of them with a subchart.

Assuming we need the ApplicationSet to template values in the `values.yaml`, then we need to use a Git file generator instead of a directory generator. The value of the `path` key of the Git file generator should be set to:

```
path: cluster-charts/*/*/values.yaml
```

However, the default implementation will interpret the above pattern as:

```
path: cluster-charts/**/values.yaml
```

Meaning, for `mychart` in `cluster1`, that it will pick up both the chart's `values.yaml` but also the one from its subchart. This will most likely fail, and even if it didn't it would be wrong.

There are multiple other ways this undesirable globbing can fail. For example:

```
path: some-path/*.yaml
```

This will return all YAML files in any directory at any level under `some-path`, instead of only those directly under it.

## Enabling the New Globbing

Since some users may rely on the old behavior it was decided to make the fix optional and not enabled by default.

It can be enabled in any of these ways:

1. Pass `--enable-new-git-file-globbing` to the ApplicationSet controller args.
1. Set `ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_NEW_GIT_FILE_GLOBBING=true` in the ApplicationSet controller environment variables.
1. Set `applicationsetcontroller.enable.new.git.file.globbing: "true"` in the `argocd-cmd-params-cm` ConfigMap.

Note that the default may change in the future.

## Usage

The new Git file generator globbing uses the `doublestar` package. You can find it [here](https://github.com/bmatcuk/doublestar).

Below is a short excerpt from its documentation.

doublestar patterns match files and directories recursively. For example, if
you had the following directory structure:

```bash
grandparent
`-- parent
    |-- child1
    `-- child2
```

You could find the children with patterns such as: `**/child*`,
`grandparent/**/child?`, `**/parent/*`, or even just `**` by itself (which will
return all files and directories recursively).

Bash's globstar is doublestar's inspiration and, as such, works similarly.
Note that the doublestar must appear as a path component by itself. A pattern
such as `/path**` is invalid and will be treated the same as `/path*`, but
`/path*/**` should achieve the desired result. Additionally, `/path/**` will
match all directories and files under the path directory, but `/path/**/` will
only match directories.
