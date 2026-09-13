
# Git Configuration

## System Configuration

Argo CD uses the Git installation from its base image (Ubuntu), which
includes a standard system configuration file located at
`/etc/gitconfig`. This file is minimal, just defining filters
necessary for Git LFS functionality.

You can customize Git's system configuration by mounting a file from a
ConfigMap or by creating a custom Argo CD image.

## Global Configuration

Argo CD runs Git with the `HOME` environment variable set to
`/dev/null`. As a result, global Git configuration is not supported.

## Built-in Configuration

The `argocd-repo-server` adds specific configuration parameters to the
Git environment to ensure proper Argo CD operation. These built-in
settings override any conflicting values from the system Git
configuration.

Currently, the following built-in configuration options are set:

- `maintenance.autoDetach=false`
- `gc.autoDetach=false`

These settings force Git's repository maintenance tasks to run in the
foreground. This prevents Git from running detached background
processes that could modify the repository and interfere with
subsequent Git invocations from `argocd-repo-server`.

You can disable these built-in settings by setting the
`argocd-cmd-params-cm` value `reposerver.enable.builtin.git.config` to
`"false"`. This allows you to experiment with background processing or
if you are certain that concurrency issues will not occur in your
environment.

> [!NOTE]
> Disabling this is not recommended and is not supported!

## Locale

Every Git command `argocd-repo-server` runs is given `LC_ALL=C`. Git translates its
own messages from the inherited locale, and Argo CD matches one of those messages to
recover a repository wedged by a leftover lock file (see below), so they must stay in
the form it parses. This is set unconditionally and is not affected by
`reposerver.enable.builtin.git.config`.

A side effect is that Git output in `argocd-repo-server` logs is always in English,
regardless of the container's locale.

## Recovering from an interrupted Git process

Git takes a lock by creating a file such as `.git/HEAD.lock` and renaming it over its
target. It removes that file on every exit it controls, so a leftover lock is only
possible when the process is killed outright:

```text
who removes a Git lock file

 success    create ──▶ write ──▶ rename over the target ──▶ gone
                                 the rename consumes the lock

 error      create ──▶ write ──▶ unlink                 ──▶ gone
                                 Git's own cleanup path

 SIGTERM    create ──▶ write ──▶ signal handler unlinks ──▶ gone

 SIGKILL    create ──▶ write ──▶ process dies           ──▶ STAYS
                                 no cleanup code runs
```

An OOM kill, a lost node, or a cancelled request delivers that `SIGKILL`. The lock is
a plain file rather than a kernel lock, so nothing reclaims it, and every later fetch
or checkout on that cached repository fails with:

```
Unable to create '<path>/.git/HEAD.lock': File exists
```

Left alone this does not resolve. `argocd-repo-server` keeps passing its health check
while every Application backed by that repository stops reconciling.

`argocd-repo-server` therefore recovers from it: when a fetch or checkout fails with
that error, the lock named in the error is removed and the operation is retried once.
Locks are only removed when they can be shown to be leftovers:

```text
what argocd-repo-server will and will not remove

  a fetch or checkout fails with "Unable to create '…': File exists"
                        │
                        ▼
  a *.lock inside this repository's own .git/  no ──▶ left alone,
                        │ yes                          error returned as-is
                        ▼
  a regular file, not a symlink                no ──▶ left alone, logged
                        │ yes                          "git lock files are
                        ▼                               regular files"
  still the same file that was inspected       no ──▶ left alone, logged
                        │ yes                          "it was replaced while
                        ▼                               being inspected"
  older than twice ARGOCD_EXEC_TIMEOUT         no ──▶ left alone, logged
                        │ yes                          "age … is within the
                        ▼                               … grace period"
            removed, and the operation retried once
```

The age requirement is what separates a leftover lock from one a running Git process
still holds. Removing a live lock lets two Git processes write the same working tree,
leaving it holding a mix of two revisions with no error raised anywhere — worse than
the stuck repository it would be fixing.

Two consequences worth knowing:

- **Recovery is not immediate.** A lock is left alone until it is older than twice
  `ARGOCD_EXEC_TIMEOUT` (at least one minute), so a reconcile can still fail in the
  meantime. Each declined removal is logged with the lock's age and the window.
- **`ARGOCD_EXEC_TIMEOUT=0` disables the recovery.** With no timeout Git is never
  killed, so no elapsed time can establish that a lock is dead. This is logged when it
  occurs.
