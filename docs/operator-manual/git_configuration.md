
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

## Sparse Checkout (Partial Clone)

For very large repositories where Argo CD only needs a small subset of the
tree, you can configure sparse paths on a repository. When sparse paths are
configured, `argocd-repo-server` performs a partial clone (`--filter=blob:none`)
and uses Git's [sparse checkout][1] (cone mode) so that only the matching paths
are materialised on disk. Blobs outside the configured paths are fetched lazily
on demand from the promisor remote.

### When to use it

- Mono-repos where a single Argo CD instance serves many Applications, each
  scoped to a different subdirectory.
- Repositories with large binary or generated files outside of the directories
  Argo CD actually renders.

If your Applications already render most of the repository, sparse checkout
will not save meaningful bandwidth or disk space.

Sparse paths are set per repository, not per Application. Every Application
that uses the repository URL sees only the configured paths, so the list must
cover the directories of all of them, including any Helm value files or
Kustomize bases they reference elsewhere in the repository.

> [!WARNING]
> Adding sparse paths to a repository that existing Applications already use
> makes any Application whose path reaches outside the configured paths render
> a partial set of manifests. With automated sync and pruning enabled, Argo CD
> then deletes the live resources that are missing from those manifests.
> Check every Application using the repository before adding or narrowing
> sparse paths.

### Enabling

You can configure sparse paths through the CLI, the UI, or a Repository secret.

**CLI:**

```bash
argocd repo add https://github.com/example/mono-repo.git \
    --sparse-paths charts/my-app \
    --sparse-paths environments/production
```

Each `--sparse-paths` flag adds one path. Specifying any sparse paths implies
that a partial clone will be used for this repository.

**UI:** the *Sparse paths (optional, comma-separated)* field on the
*Connect Repo* form (Git type only) accepts a comma-separated list of paths.

**Repository secret:** add a `sparsePaths` data key with a comma-separated
list of paths:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: example-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  url: https://github.com/example/mono-repo.git
  sparsePaths: charts/my-app,environments/production
```

Paths must be directories relative to the repository root (e.g. `charts` or
`apps/foo`). Cone mode (`git sparse-checkout init --cone --sparse-index`) is
used, so each entry must be a directory rather than a glob. Leading and
trailing slashes are stripped. Globs, paths that start with `-`, and paths
that reference a parent directory (`..`) are rejected when the repository is
created or updated. If Git still refuses the configuration (for example
because an entry names a file), manifest generation fails with an error
rather than falling back to a full checkout.

### Incompatibilities and caveats

> [!WARNING]
> **Git LFS:** Partial clone is incompatible with Git LFS, so a repository
> cannot have both sparse paths and `enableLfs` set. The API rejects the
> combination, and manifest generation fails if it is set through a
> Repository secret.

> [!NOTE]
> **Server-side support:** Partial clone requires the Git server to advertise
> the `filter` capability. All major hosted providers (GitHub, GitLab,
> Bitbucket Server/Cloud, Azure DevOps, Gitea) support this on recent
> versions. Self-hosted servers running older Git releases may not.

> [!NOTE]
> **Pre-fetch fallback:** Argo CD attempts to batch-fetch the blobs needed for
> the configured paths up front. If the pre-fetch fails (for example because
> the remote refuses a long object list), Argo CD logs a warning and falls
> back to lazy on-demand fetching during checkout — the operation still
> completes correctly, just with more round trips.

[1]: https://git-scm.com/docs/git-sparse-checkout
