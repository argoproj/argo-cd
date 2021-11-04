# Argo CD UI

<img src="https://github.com/argoproj/argo-cd/blob/master/ui/src/assets/images/argo.png?raw=true" alt="Argo Image" width="600" />

Web UI for [Argo CD](https://github.com/argoproj/argo-cd).


## Getting started

  1. Install [NodeJS](https://nodejs.org/en/download/) and [pnpm](https://pnpm.io).  On macOS with [Homebrew](https://brew.sh/), running `brew install node pnpm` will accomplish this.
  2. Run `pnpm install` to install local prerequisites.
  3. Run `pnpm start` to launch the webpack dev UI server.
  4. Run `pnpm build` to bundle static resources into the `./dist` directory.

To build a Docker image, run `IMAGE_NAMESPACE=yourimagerepo IMAGE_TAG=latest pnpm docker`.

To do the same and push to a Docker registry, run `IMAGE_NAMESPACE=yourimagerepo IMAGE_TAG=latest DOCKER_PUSH=true pnpm docker`.

## Pre-commit Checks

Make sure your code passes the lint checks:

```
pnpm lint --fix
```