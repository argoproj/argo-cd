# Releasing

## Automated release procedure

Starting from `release-1.6` branch, ArgoCD can be released in an automated fashion
using GitHub actions. The release process takes about 20 minutes, sometimes a
little less, depending on the performance of GitHub Actions runners.

The target release branch must already exist in the GitHub repository. If you for
example want to create a release `v1.7.0`, the corresponding release branch
`release-1.7` needs to exist, otherwise, the release cannot be built. Also,
the trigger tag should always be created in the release branch, checked out
in your local repository clone.

Before triggering the release automation, the `CHANGELOG.md` should be updated
with the latest information, and this change should be committed and pushed to
the GitHub repository to the release branch. Afterward, the automation can be
triggered.

**Manual steps before release creation:**

* Update `CHANGELOG.md` with changes for this release
* Commit & push changes to `CHANGELOG.md`
* Prepare release notes (save to some file, or copy from Changelog)

**The automation will perform the following steps:**

* Update `VERSION` file in the release branch
* Update manifests with image tags of the new version in the release branch
* Build the Docker image and push to Docker Hub
* Create a release tag in the GitHub repository
* Create a GitHub release and attach the required assets to it (CLI binaries, ...)

Finally, it will the remove trigger tag from the repository again.

Automation supports both, GA and pre-releases. The automation is triggered by
pushing a tag to the repository. The tag must be in one of the following formats
to trigger the GH workflow:

* GA: `release-v<MAJOR>.<MINOR>.<PATCH>`
* Pre-release: `release-v<MAJOR>.<MINOR>.<PATCH>-rc<RC#>`

The tag must be an annotated tag, and it must contain the release notes in the
commit message. Please note that Markdown uses `#` character for formatting, but
Git uses it as comment char. To solve this, temporarily switch Git's comment char
to something else, the `;` character is recommended.

For example, consider you have configured the Git remote for the repository to
`github.com/argoproj/argo-cd` to be named `upstream` and are in your locally
checked out repo:

```shell
git config core.commentChar ';'
git tag -a -F /path/to/release-notes.txt release-v1.6.0-rc2
git push upstream release-v1.6.0-rc2
git tag -d release-v1.6.0-rc2
git config core.commentChar '#'

```

For convenience, there is a shell script in the tree that ensures all the
pre-requisites are met and that the trigger is well-formed before pushing
it to the GitHub repo.

In summary, the modifications it does are:

* Create annotated trigger tag in your local repository
* Push the tag to the GitHub repository to trigger the workflow
* Remove trigger tag from your local repository

The script can be found at `hacks/trigger-release.sh` and is used as follows:

```shell
./hacks/trigger-release.sh <version> <remote name> [<release notes path>]
```

The `<version>` identifier needs to be specified **without** the `release-`
prefix, so just specify it as `v1.6.0-rc2` for example. The `<remote name>`
specifies the name of the remote used to push to the GitHub repository. 

If you omit the `<release notes path>`, an editor will pop-up asking you to
enter the tag's annotation so you can paste the release notes, save, and exit.
It will also take care of temporarily configuring the `core.commentChar` and
setting it back to its original state.

:warning:
    It is strongly recommended to use this script to trigger the workflow
    instead of manually pushing a tag to the repository.

Once the trigger tag is pushed to the repo, the GitHub workflow will start
execution. You can follow its progress under the `Actions` tab, the name of the
action is `Create release`. Don't get confused by the name of the running
workflow, it will be the commit message of the latest commit to the `master`
branch, this is a limitation of GH actions.

The workflow performs necessary checks so that the release can be successfully
built before the build actually starts. It will error when one of the
prerequisites is not met, or if the release cannot be built (i.e. already
exists, release notes invalid, etc etc). You can see a summary of what has
failed in the job's overview page and more detailed errors in the output
of the step that has failed.

:warning:
    You cannot perform more than one release on the same release branch at the
    same time. For example, both `v1.6.0` and `v1.6.1` would operate on the
    `release-1.6` branch. If you submit `v1.6.1` while `v1.6.0` is still
    executing, the release automation will not execute. You have to either
    cancel `v1.6.0` before submitting `v1.6.1` or wait until it has finished.
    You can execute releases on different release branches simultaneously, for
    example, `v1.6.0` and `v1.7.0-rc1`, without problems.

### Verifying automated release

After the automatic release creation has finished, you should perform manual
checks to see if the release came out correctly:

* Check status & output of the GitHub action
* Check [https://github.com/argoproj/argo-cd/releases](https://github.com/argoproj/argo-cd/releases)
  to see if the release has been correctly created and if all required assets
  are attached.
* Check whether the image has been published on DockerHub correctly

### If something went wrong

If something went wrong, damage should be limited. Depending on the steps that
have been performed, you will need to manually clean up.

* Delete the release tag (e.g. `v1.6.0-rc2`) created in the GitHub repository. This
  will immediately set the release (if created) to `draft` status, invisible to the
  general public.
* Delete the draft release (if created) from the `Releases` page on GitHub
* If Docker image has been pushed to DockerHub, delete it
* If commits have been performed to the release branch, revert them. Paths that could have been committed to are:
    * `VERSION`
    * `manifests/*`

### Post-process manual steps

For now, the only manual steps left are to

* update stable tag in the GitHub repository to point to new the release (if appropriate)
* update the `VERSION` file on `master` if this is a new major release

These may be automated as well in the future.

## Manual releasing

The automatic release process does not interfere with the manual release process, since
the trigger tag does not match a normal release tag. If you prefer to perform,
manual release or if automatic release is for some reason broken, these are the
steps:

Make sure you are logged into Docker Hub:

```bash
docker login
```

Export the upstream repository and branch name, e.g.:

```bash
REPO=upstream ;# or origin 
BRANCH=release-1.3
```

Set the `VERSION` environment variable:

```bash 
# release candidate
VERSION=v1.3.0-rc1
# GA release
VERSION=v1.3.1
```

Update `VERSION` and manifests with the new version:

```bash
git checkout $BRANCH
echo ${VERSION:1} > VERSION
make dev-tools-image
make manifests IMAGE_TAG=$VERSION
git commit -am "Update manifests to $VERSION"
git tag $VERSION
```

Build, and push release to Docker Hub

```bash
git clean -fd
make release IMAGE_NAMESPACE=argoproj IMAGE_TAG=$VERSION DOCKER_PUSH=true
git push $REPO $BRANCH
git push $REPO $VERSION
```

Update [GitHub releases](https://github.com/argoproj/argo-cd/releases) with:

* Getting started (copy from the previous release)
* Changelog
* Binaries (e.g. `dist/argocd-darwin-amd64`).

## Update brew formulae (manual)

If GA, update the Brew formula:

```bash
brew bump-formula-pr argocd --version ${VERSION:1}
```

## Update stable tag (manual)

If GA, update `stable` tag:

```bash
git tag stable --force && git push $REPO stable --force
```

## Verify release

Locally:

```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/$VERSION/manifests/install.yaml
```

Follow the [Getting Started Guide](../getting_started/).

If GA:

```bash
brew upgrade argocd
/usr/local/bin/argocd version
```

Sync Argo CD in [https://cd.apps.argoproj.io/applications/argo-cd](https://cd.apps.argoproj.io/applications/argo-cd).

Deploy the [site](site.md).
