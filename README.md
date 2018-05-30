# ArgoCD UI

![Argo Image](https://github.com/argoproj/argo/blob/master/argo.png?raw=true)

Web UI for [Argo CD](https://github.com/argoproj/argo-cd).


## Getting started

  1. Install [NodeJS](https://nodejs.org/en/download/) and [Yarn](https://yarnpkg.com).  On macOS, this can be done by running `brew install node yarn`.
  2. Install local prerequisites by running `npm install`.
  3. Run the webpack dev UI server by running `yarn start`.
  4. Build static resources and bundle them into the `./dist` directory by running `yarn build`.

To build a Docker image, run `IMAGE_NAMESPACE=yourimagerepo IMAGE_TAG=latest yarn docker`.

To do the same and push to a Docker registry, run `IMAGE_NAMESPACE=yourimagerepo IMAGE_TAG=latest DOCKER_PUSH=true yarn docker`.
