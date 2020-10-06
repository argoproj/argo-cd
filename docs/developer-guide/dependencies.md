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
