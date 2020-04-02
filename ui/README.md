# Argo CD UI

<img src="https://github.com/argoproj/argo-cd/blob/master/ui/src/assets/images/argo.png?raw=true" alt="Argo Image" width="600" />

Web UI for [Argo CD](https://github.com/argoproj/argo-cd).


## Getting started

  1. Install [NodeJS](https://nodejs.org/en/download/) and [Yarn](https://yarnpkg.com).  On macOS with [Homebrew](https://brew.sh/), running `brew install node yarn` will accomplish this.
  2. Run `yarn install` to install local prerequisites.
  3. Run `yarn start` to launch the webpack dev UI server.
  4. Run `yarn build` to bundle static resources into the `./dist` directory.

To build a Docker image, run `IMAGE_NAMESPACE=yourimagerepo IMAGE_TAG=latest yarn docker`.

To do the same and push to a Docker registry, run `IMAGE_NAMESPACE=yourimagerepo IMAGE_TAG=latest DOCKER_PUSH=true yarn docker`.

## Pre-commit Checks

Make sure your code passes the lint checks:

```
yarn lint --fix
```