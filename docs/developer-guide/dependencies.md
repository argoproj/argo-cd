# Managing Dependencies

## Notifications Engine (`github.com/argoproj/notifications-engine`)

### Repository

[notifications-engine](https://github.com/argoproj/notifications-engine)

### Pulling changes from `notifications-engine`

After your Notifications Engine PR has been merged, ArgoCD needs to be updated to pull in the version of the notifications engine that contains your change. Here are the steps:

- Retrieve the SHA hash for your commit. You will use this in the next step.
- From the `argo-cd` folder, run the following command

  `go get github.com/argoproj/notifications-engine@<git-commit-sha>`

  If you get an error message `invalid version: unknown revision` then you got the wrong SHA hash

- Run:

  `go mod tidy`

- The following files are changed:

  - `go.mod`
  - `go.sum`

- If your notifications engine PR included docs changes, run `make codegen` or `make codegen-local`.

- Create an ArgoCD PR with a `refactor:` type in its title for the above file changes.

## Argo UI Components (`github.com/argoproj/argo-ui`)
### Contributing to Argo CD UI

Argo CD, along with Argo Workflows, uses shared React components from [Argo UI](https://github.com/argoproj/argo-ui). Examples of some of these components include buttons, containers, form controls, 
and others. Although you can make changes to these files and run them locally, in order to have these changes added to the Argo CD repo, you will need to follow these steps. 

1. Fork and clone the [Argo UI repository](https://github.com/argoproj/argo-ui).

2. `cd` into your `argo-ui` directory, and then run `yarn install`. 

3. Make your file changes.

4. Run `yarn start` to start a [storybook](https://storybook.js.org/) dev server and view the components in your browser. Make sure all your changes work as expected. 

5. Use [yarn link](https://classic.yarnpkg.com/en/docs/cli/link/) to link Argo UI package to your Argo CD repository. (Commands below assume that `argo-ui` and `argo-cd` are both located within the same parent folder)

    * `cd argo-ui`
    * `yarn link`
    * `cd ../argo-cd/ui`
    * `yarn link argo-ui`

    Once the `argo-ui` package has been successfully linked, test changes in your local development environment. 

6. Commit changes and open a PR to [Argo UI](https://github.com/argoproj/argo-ui). 

7. Once your PR has been merged in Argo UI, `cd` into your `argo-cd/ui` folder and run `yarn add git+https://github.com/argoproj/argo-ui.git`. This will update the commit SHA in the `ui/yarn.lock` file to use the latest master commit for argo-ui. 

8. Submit changes to `ui/yarn.lock`in a PR to Argo CD. 
