# Managing Dependencies

## GitOps Engine (`github.com/argoproj/gitops-engine`)

### Repository

https://github.com/argoproj/gitops-engine

### Pulling changes from `gitops-engine` 

After your GitOps Engine PR has been merged, ArgoCD needs to be updated to pull in the version of the GitOps engine that contains your change.  Here are the steps:

* Retrieve the SHA hash for your commit. You will use this in the next step.
* From the `argo-cd` folder, run the following command

    `go get github.com/argoproj/gitops-engine@<git-commit-sha>`

    If you get an error message `invalid version: unknown revision` then you got the wrong SHA hash

* Run:

    `go mod tidy`

* The following files are changed:

    - `go.mod`
    - `go.sum`

* Create an ArgoCD PR with a `refactor:` type in its title for the two file changes.

### Tips:
* See https://github.com/argoproj/argo-cd/pull/4434 as an example
* The PR might require additional, dependent changes in ArgoCD that are directly impacted by the changes made in the engine.  

## Argo UI Components

### Repository

https://github.com/argoproj/argo-ui

### Pulling changes from Argo UI into Argo CD

If you make changes to the Argo UI component, and your Argo CD changes depend on those changes, follow these steps:

1. Make changes to Argo UI and submit the PR request.
2. Also, prepare your Argo CD changes, but don't create the PR just yet.
3. **After** the Argo UI PR has been merged to master, then as part of your Argo CD changes:
	- Run `yarn add https://github.com/argoproj/argo-ui.git`, and then,
	- Check in the regenerated yarn.lock file as part of your Argo CD commit
4. Create the Argo CD PR	 when you are ready. The PR build and test checks should pass.

If your Argo UI change is a 'stand-alone' fix, and you simply want Argo CD to pull in your change, then simply create an Argo CD PR with the yarn.lock file change.


