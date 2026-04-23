---
title: Tool Execution Sandbox
authors:
  - "@dudinea"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-04-23
last-updated: 2026-04-23
---

# Tool Execution Sandbox

A mechanism to restrict the filesystem access, network connectivity, and system
call surface of tools executed by `argocd-repo-server` and `argocd-cmp-server`
(Helm, Kustomize, Config Management Plugins) using Linux kernel security
primitives.

## Summary

`argocd-repo-server` and `argocd-cmp-server` execute external binaries
(Helm, Kustomize, and arbitrary CMP plugin scripts) with the same
privileges and filesystem visibility as the server process itself.
Those tools can therefore inadvertently or maliciously read secrets,
traverse the repository cache of other applications, or make
unexpected outbound network calls.

This proposal introduces a new `argocd-sandbox` command and an accompanying
configuration mechanism that wraps tools invocations inside a security
sandbox. The sandbox is configured in two stages: a static per-tool
configuration file that describes the baseline allowed resources, and
per-invocation command-line flags that add the run-specific paths (cloned
repository directory, temporary working directory, etc.). `argocd-repo-server`
and `argocd-cmp-server` are extended with new configuration options to enable
sandbox execution for each supported tool.

The initial sandbox implementation will use the
[Landlock](https://docs.kernel.org/userspace-api/landlock.html)
security mechanism, which requires no privileged setup, no kernel
module or extra k8s-level configurations.

## Motivation

### Current security situation

When Argo CD renders manifests for an application, it executes a subprocess in
the cloned repository directory.  That subprocess:

1. Can read any file accessible to the `argocd-repo-server` or
   `argocd-cmp-server` process — including the repository caches of
   other applications, plugin sockets, and mounted secrets.
2. Can make outbound network connections to arbitrary endpoints.
3. Can invoke any system call permitted to the server process.

The CMP v2 model (sidecar containers) is a significant improvement
because the plugin tooling runs in a separate container. However,
repository content is always transmitted as a tar.gz archive over a
gRPC stream, adding significant latency and CPU overhead for big
repositories, while the plugin can still access all paths visible
inside its own container, including temporary data of concurent
invocations on behalf of other applications.

### Goals

- Restrict each tool invocation to only the filesystem paths and operations it
  legitimately needs. 
- Optionally block outbound network access for tools that do not require it.
- Provide an opt-out based configuration mechanism that is
  backwards-compatible with existing deployments (may be opt-in in
  initial versions)
- Enable implementation of folow-on tasks, such as:
  + Enable safe sharing of the repository volume between
	`argocd-repo-server` and `argocd-cmp-server` (eliminating the
	tar-over-gRPC overhead) because each plugin invocation will be
	confined to its own repository sub-tree.
  + Enable safe re-introduction of Jsonnet as a built-in manifest
    generator without risk from resource-exhaustion or unbounded
    loops.
  + Enable implementation of additional sandbox mechanisms, such as
    seccomp to filter dangerous system calls to reduce the
    kernel attack surface and user namespace support for fine graned
	security/resource usage configuration.
  

### Non-Goals

- Providing Windows or macOS support for the sandbox primitives (those
  platforms may lack the required kernel APIs).
- Implementing cgroup-based CPU/memory limits in the initial version.
- Limiting filesystem or network access of the argocd-repo-server and
  other argocd server processes.

## Proposal

### Overview

A new `argocd-sandbox` command acts as a *trampoline*: it receives a
description of the desired sandbox environment, applies the kernel-level
restrictions to the current process, and then `exec()`s the real tool binary,
replacing itself.  The tool therefore inherits all restrictions and runs with
no awareness of the sandbox layer.

```
argocd-repo-server
  └─ exec ──► argocd-sandbox --impl landlock 
                             --landlock-config /home/argocd/sandbox/landlock/helm.yaml
                             --landlock-allow fs:r:/tmp/_argocd-repo/a1b2c3d4-e5f6-7890-abcd-ef1234567890
                             --landlock-allow fs:rw:/tmp/helm-work-xyz
                             -- helm template /tmp/_argocd-repo/a1b2c3d4-e5f6-7890-abcd-ef1234567890/myapp
                └─ exec ──► helm  [restricted process]
```

### Use cases

Some example use cases:

- `helm template` invocations are restricted to the cloned directory,
  preventing it from reading secrets, other repositories contents or
  making unexpected network calls in case this tool has some
  unmiticated remote execution CVEs.
- Each CMP plugin invocation is confined to its own repository sub-tree and a
  scratch directory, preventing cross-application data access within a shared
  sidecar.
- Kustomize may be prevented from calling external URLs if this
  contradicts the corporate policy


### Implementation Details

#### The `argocd-sandbox` command

The command has the following interface:

```
argocd-sandbox [sandbox-flags] -- <tool-binary> [tool-args...]
```

**Sandbox flags:**

| Flag                                    | Description                                                                                                                                            |
|-----------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------|
| '--impl                                 | Sandbox type. May be 'landlock' or 'none' in the initial implementation.                                                                               |
| `--config <path>`                       | Path to the static sandbox configuration file. If omitted, a built-in safe default is used.                                                            |
| `--<impl>-allow <type>:<path>:<access>` | Grant `<type>` `<access>` to `<path>`. For the initial implementation type will be "fs", `<access>` is one of `r`, `rw`, `rx`, `rwx`. May be repeated. |


The `--` separator unambiguously marks the end of sandbox flags and
the beginning of the tool command. Every argument after `--` is passed
unchanged to `exec()` call.

**Static configuration file**

```yaml
# /home/argocd/sandbox/helm.yaml
# default limitations
landlock:             # landlock plugin configuration
  defaultFSDeny: "*"  # deny all FS operations
  defaultNetDeny:     # deny binding to ports while allowing outbound connections
	- BindTCP

  # default allowed paths
  allowedPaths:
	# Helm static binary 
	- path: /usr/local/bin/helm
	  access: rx
	# OS trust store — required for TLS when outbound is allowed
	- path: /etc/ssl/certs
	  access: r
	- path: /etc/resolv.conf
	  access: r
```

Per-invocation paths (repository directory, per-request temp directory) are
supplied as `--landlock-allow` flags by `argocd-repo-server` or `argocd-cmp-server`
at call time and are merged with the static configuration.

#### Sandbox backend: Landlock (initial implementation)

[Landlock](https://landlock.io) is a Linux Security Module available
since kernel 5.13 (from 2021) that allows an unprivileged process to
restrict its own filesystem access.  Unlike mandatory access control
systems (SELinux, AppArmor), Landlock requires no privileged setup and
no kernel module configuration — the process restricts itself.

Crucially, Landlock requires neither `CAP_SYS_ADMIN` nor user
namespace support enabled. Any unprivileged process can apply it to
itself without any special Pod security context settings or
cluster-level policy changes, meaning it works out of the box on the
vast majority of Kubernetes installations.

The implementation will use the
[`landlock-lsm/go-landlock`](https://github.com/landlock-lsm/go-landlock) Go
package, which handles Landlock ABI version negotiation and graceful
degradation across kernel versions transparently.

Setup sequence inside `argocd-sandbox`:

1. Create a ruleset specifying the filesystem access rights to be mediated
   (read, write, execute, etc.).
2. Add one rule per path entry from the merged static and per-invocation
   configuration.
3. Activate the restrictions on the current process.
4. Replace the current process image with the tool binary. The kernel
   propagates Landlock restrictions through `exec()`, so the tool starts
   already confined with no opportunity to escape.

The feature degrades gracefully: if the running kernel does not support
Landlock, `argocd-sandbox` can either abort with a clear error or log a
warning and fall back to `none` mode depending if operator enabled `none`
`--sandbox-impl`.

#### Why re-execution trampoline rather than inline setup?

Go's runtime starts multiple OS threads before `main()` is reached.
The `fork()` call in a multi-threaded Go process duplicates only the
calling thread into the child (POSIX semantics), which means the child
process is in an undefined state with respect to Go's internal lock
state, garbage collector, and goroutine scheduler.  Calling Go runtime
functions between `fork()` and `exec()` is therefore unsafe — the only
safe operations are C library calls and direct syscalls, thus
requiring essentially to write some unsafe low level C-like code in go or
to call some C code with all the trouble that it brings.

`os/exec` in Go calls `fork()` + `exec()` in a tight sequence with
only the minimal `SysProcAttr`- specified kernel operations occurring
between the two calls, before the Go runtime re-initialises.  There is
no support for injecting arbitrary Go code or configuring the Landlock
security module using `SysProcAttr`.

The re-execution model solves this cleanly:

- `argocd-repo-server`/`argocd-cmp-server` use `os/exec` to start
  `argocd-sandbox ...` as a normal child process.  No custom fork/exec
  code is needed.
- `argocd-sandbox` limits itself and then calls `exec()` to replace
  itself with the tool and the kernel carries all Landlock/seccomp
  state into the new image.
- The `argocd-sandbox` startup is expected to be fast without
  requiring additional IO because it uses the same Argo CD binary that
  is already loaded into memory.



#### Additional sandbox backends (future)

| Backend                       | Notes                                                       |
|-------------------------------|-------------------------------------------------------------|
| seccomp                       | Syscall filtering according,  can be combined with Landlock |
| User namespaces + bind mounts | Filesystem view limited to explicit bind mounts             |
| cgroups-v2                    | Resource limits for tools                                   |

#### Integration with `argocd-repo-server`

A new section is added to the `argocd-repo-server` configuration (in
`argocd-cmd-params-cm` 

```yaml
# argocd-cmd.params  ConfigMap
data:
  # Helm sandbox
  reposerver.helm.sandbox.enabled: "true"
  reposerver.helm.sandbox.config: ""                    # empty = built-in default
  reposerver.helm.sandbox.impl: "landlock,none"         # landlock with fallback to none, optional with default applied if not set.

  # Kustomize sandbox
  reposerver.kustomize.sandbox.enabled: "true"
  reposerver.kustomize.sandbox.config: ""               # empty = built-in default
  reposerver.kustomize.sandbox.impl: "landlock,none"    # landlock with fallback to none, optional with default applied if not set.

```

#### Integration with `argocd-cmp-server`

The `ConfigManagementPlugin` specification gains an optional `sandbox` field:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  name: my-plugin
spec:
  generate:
    command: [my-tool, generate]
  sandbox:
    enabled: true              # optional; inherits server default if omitted
    impl: landlock             # optional; inherits server default if omitted
    config: /plugin/sandbox.yaml  # optional; path inside the CMP container
```

When `sandbox.enabled` is true, `argocd-cmp-server` wraps the `generate`
(and `init`) commands with `argocd-sandbox`, passing `--allow` flags for the
request-specific repository directory and a temporary directory.

### New possibilities enabled by the sandbox

#### Safe shared repository volume between repo-server and CMP server

Currently, repository content is transmitted from `argocd-repo-server` to each
`argocd-cmp-server` sidecar as a gzip-compressed tar archive over gRPC for
every manifest generation request.  For large repositories this is a
significant source of latency and CPU overhead.

The CMP v2 design deliberately avoided a shared volume because there was no
mechanism to prevent a plugin from reading repositories belonging to other
applications.  The tool execution sandbox removes this obstacle: each CMP
invocation is confined by Landlock to a specific repository sub-tree, so a
shared volume is safe.

With the sandbox in place, the architecture can be extended as follows:

1. `argocd-repo-server` mounts the repository checkout directory into a shared
   `emptyDir` volume (e.g., `/tmp/_argocd-repo`).
2. `argocd-cmp-server` mounts the same volume at the same path.
3. When invoking the plugin, `argocd-cmp-server` passes repository directory
   `--allow /tmp/_argocd-repo/<repo-uuid>:read` and temporary directory
   `--allow /tmp/plugin-work-<reqid>:readwrite` to `argocd-sandbox`.
4. The plugin tool can read the repository directly from the shared volume
   without any serialisation/deserialisation step.

This eliminates the archive transfer entirely for CMP plugins and is expected
to provide a substantial latency reduction for large monorepos. 

The current (tar.gz based) mechanism may stay as an available
alternative (enabled in the plugin configuration file) because in some
cases (very long running plugins, for example making network requests)
it is preferable because the repository gets locked only while the
`tar.gz` file is being created, thus reducing lock contention on the
repo.


#### Safer re-introduction of Jsonnet as a built-in tool

With the sandbox it will be possible to move Jsonnet support out from
the argocd-repo-server into an external tool or CMP plugin (`argocd-jsonnet-runner`)
without significantly sacrifying performance and getting security advantages:

- Filesystem writes are restricted to the designated temporary directory;
  Jsonnet cannot fill arbitrary paths on the repo-server's disk.
- Network access may be disabled, preventing import of remote Jsonnet libraries not
  already present in the repository.
- The resource usage is limited by the argocd-cmp-server container
  resource limits or future integration with cgroup-v2 limits


## Drawbacks

- Adds a new Linux-specific code path that does not function on
  non-Linux platforms (macOS, Windows). This is acceptable because
  Argo CD's production deployment target is Linux containers, but it
  increases the complexity of local development environments and
  requires running unit tests with the conteinerized development
  toolchain.
- Operators must craft and maintain sandbox configuration files for custom
  tools and CMP plugins. The built-in defaults for Helm, Kustomize will 
  reduce the burden for the common case.
- The sandbox cannot prevent a tool from consuming excessive CPU or memory in
  the initial implementation (no cgroup integration). 

## Alternatives

### UID-per-repository isolation (existing proposal)

The CMP v2 proposal mentions using unique UIDs per cloned repository to prevent
out-of-tree access. This approach:

- Requires elevated privileges or a privileged helper.
- Provides filesystem isolation only — no network or syscall restrictions.

Landlock achieves the same filesystem isolation without elevated privileges
and additionally allows network restrictions.

### Using existing command line tools for sandboxing

There are some command line tools like:

* [nsjail](https://github.com/google/nsjail),
  [bubblewrap](https://github.com/GoogleChromeLabs/bubblewrap) -
  sandboxing untrusted binaries with fine-grained filesystem, syscall,
  and resource controls. requires CAP_SYS_ADMIN (or user namespaces)
* [landrun](https://github.com/Zouuup/landrun) - Landlock based sandboxing.
* Probably more

It is possible to add those tools instead of writing argocd-sandbox.
However using those tools means relying on many external dependencies,
complex integration of thise tools with argocd componentsm, and
possible longer startup times for tools, especially if we need to
combine several tools.

