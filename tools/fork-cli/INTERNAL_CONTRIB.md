# Fork CLI: ArgoCD Internal Contribution Guide

This guide explains how to work with our ArgoCD fork at Skyscanner. There are two main workflows:

1. **Internal Development** - Developing fixes/features for internal use at Skyscanner
2. **Upstream Contribution** - Contributing our changes back to ArgoCD upstream

## Branch Structure

Our fork maintains several important branches:

- **`skyscanner-internal/master`**
  - Default branch of our fork (`skyscanner/argo-cd`)
  - Contains latest upstream code + Skyscanner CI files
  - Should contain minimal changes (mainly CI/build configs and tooling)
  - Updated by rebasing onto `skyscanner-contrib/master`

- **`skyscanner-internal/develop/<release-tag>/<suffix>`**
  - Release-pinned development branches based on upstream tags
  - Format: `skyscanner-internal/develop/vX.Y.Z/fix-<name>`
  - Created by the **setup-fix** tool
  - Contains our CI folder copied from `skyscanner-internal/master` and fork tooling

- **`skyscanner-contrib/master`**
  - Mirror of `argoproj/argo-cd:master`
  - Kept in sync via automation
  - Never push to this directly

- **`skyscanner-contrib/proposal/<proposal-name>`**
  - "Clean" branch for upstream proposals
  - Contains only the commits we want to contribute upstream
  - Created by the **promote-fix** tool

## Internal Development Workflow

### Before Starting Development

1. **Ensure clean working directory**: Commit or stash any local changes
   ```shell
   git status
   # If you have changes:
   git stash push -m "WIP before sync"
   # or
   git add . && git commit -m "WIP: save local changes"
   ```

2. **Sync our fork with upstream**:
   ```shell
   go run tools/fork-cli/main.go sync-fork
   ```
   This ensures both `skyscanner-contrib/master` and `skyscanner-internal/master` are up to date.

3. **Check if the issue/feature already exists**:
   - Verify latest ArgoCD releases at https://github.com/argoproj/argo-cd/releases
   - Check ArgoCD issue tracker: https://github.com/argoproj/argo-cd/issues
   - Consider coordinating with upstream maintainers for major changes

### Starting Development

1. **Set up a fix branch** based on the release tag we're using:
   ```shell
   go run tools/fork-cli/main.go setup-fix --release="vX.Y.Z" --fix-suffix="fix-name"
   ```
   This creates `skyscanner-internal/develop/vX.Y.Z/fix-name`

   **Note**: The tool will fail if:
   - You have uncommitted changes (stash them first)
   - The release tag doesn't exist (run `git fetch --tags` or `sync-fork`)
   - Your local branches are out of date (run `sync-fork`)

2. **Create feature branches** from the development branch:
   ```shell
   go run tools/fork-cli/main.go work-on \
     --dev-branch="skyscanner-internal/develop/vX.Y.Z/fix-name" \
     --suffix="your-feature"
   ```
   This creates `feature/your-feature` and provides instructions for creating PRs.

3. **Make changes and create PRs**:
   - Make changes, commit, and push your feature branch
   - Create PRs against the development branch (not upstream!)
   - Use the `gh` CLI for convenience:
     ```shell
     gh pr create --base skyscanner-internal/develop/vX.Y.Z/fix-name \
       --title "Your feature title" \
       --body "Description of changes"
     ```

4. **Internal CI** will build and publish the image to GHCR for Skyscanner use.

## Upstream Contribution Workflow

1. **Promote your fix** to a proposal branch:
   ```shell
   go run tools/fork-cli/main.go promote-fix \
     --fix-branch="skyscanner-internal/develop/vX.Y.Z/fix-name" \
     --proposal-branch="proposal-name"
   ```

   **Note**: This tool will fail if:
   - Your `skyscanner-contrib/master` is not up to date (run `sync-fork` first)
   - The tool only creates the proposal branch now - it no longer touches `skyscanner-internal/master`

2. **Handle conflicts** during promotion:
   - The tool will guide you through resolving conflicts locally
   - In CI mode, it will tell you to run the command locally
   - Follow instructions to fix conflicts and continue

3. **Create a PR to upstream ArgoCD**:
   - Push the proposal branch: `git push origin skyscanner-contrib/proposal/proposal-name`
   - Create PR to `argoproj/argo-cd:master` from the proposal branch
   - Reference the related issue number (e.g., "Fixes #123")
   - Provide clear description of changes

4. **After upstream merge**:
   - Remove proposal branch
   - Continue using internal tag until upstream releases a version with your fix
   - Plan to migrate to the official release once available

## Command Reference

### sync-fork
Synchronizes both fork branches with upstream. **Run this regularly!**

```shell
go run tools/fork-cli/main.go sync-fork
```

**Prerequisites**: Clean working directory (no uncommitted changes)

### setup-fix
Creates a release-pinned development branch with CI tools imported.

```shell
go run tools/fork-cli/main.go setup-fix --release="v2.14.9" --fix-suffix="fix-issue-123"
```

**Prerequisites**: 
- Clean working directory
- Release tag exists
- Local branches up to date (run `sync-fork` if needed)

### work-on
Creates a feature branch off a development branch and provides PR creation guidance.

```shell
go run tools/fork-cli/main.go work-on \
  --dev-branch="skyscanner-internal/develop/v2.14.9/fix-issue-123" \
  --suffix="add-logging"
```

### promote-fix
Creates a clean proposal branch for upstream contribution.

```shell
go run tools/fork-cli/main.go promote-fix \
  --fix-branch="skyscanner-internal/develop/v2.14.9/fix-issue-123" \
  --proposal-branch="fix-issue-123"
```

**Prerequisites**: 
- `skyscanner-contrib/master` up to date (run `sync-fork` if needed)

## Best Practices

- **Always start with sync-fork** to ensure you're working with the latest code
- **Keep working directory clean** - the tools check for uncommitted changes
- **Always start from a release tag** that we're currently using
- **Verify upstream first** - check if the issue is already fixed in newer versions
- **Coordinate with upstream** before major features or changes
- **Keep internal changes minimal** - aim to contribute everything back
- **Handle conflicts locally** using the fork-cli tools
- **Use the internal build** only until upstream absorbs the change

## Troubleshooting

### "You have uncommitted changes"
```shell
# Either commit your changes:
git add . && git commit -m "WIP: description"

# Or stash them:
git stash push -m "WIP before running fork-cli"
# Later restore with: git stash pop
```

### "Tag 'vX.Y.Z' does not exist"
```shell
# Fetch latest tags:
git fetch --tags

# Or sync everything:
go run tools/fork-cli/main.go sync-fork
```

### "Branch is not up to date"
```shell
# Sync all branches:
go run tools/fork-cli/main.go sync-fork
```

### Conflicts during promotion
The tool will leave you in a cherry-pick state. Resolve conflicts manually:
```shell
# Fix conflicts in your editor, then:
git add .
git cherry-pick --continue
# Then re-run the promote-fix command
```