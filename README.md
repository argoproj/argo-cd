# ArgoCD UI

![Argo Image](https://github.com/argoproj/argo/blob/master/argo.png?raw=true)

Web UI for [Argo CD](https://github.com/argoproj/argo-cd).

## Build, run, release

* Install [NodeJS](https://nodejs.org/en/download/) and [Yarn](https://yarnpkg.com)
* Run: `yarn dev` - starts API server and webpack dev UI server. API server uses current `kubectl` context to access workflow CRDs.
* Build: `yarn build` - builds static resources into `./dist` directory.
* Release: `IMAGE_NAMESPACE=yourimagerepo IMAGE_TAG=latest DOCKER_PUSH=true yarn docker` - builds docker image and optionally push to docker registry.