
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
