# Snyk Scans

Every Sunday, Snyk scans are generated for Argo CD's `master` branch and the most recent patches of the three most
recent minor releases.

!!! note
    For the most recent scans, view the [`latest` version of the docs](https://argo-cd.readthedocs.io/en/latest/snyk/).
    You can return to your preferred version of the docs site using the dropdown selector at the top of the page.

## Scans

### master

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](master/argocd-test.html) | 0 | 0 | 1 | 0 |
| [go.mod](master/argocd-test.html) | 0 | 3 | 8 | 0 |
| [ui/package.json](master/argocd-test.html) | 0 | 6 | 5 | 0 |
| [ui/pnpm-lock.yaml](master/argocd-test.html) | 0 | 0 | 0 | 0 |
| [dex:v2.45.1](master/ghcr.io_dexidp_dex_v2.45.1.html) | 1 | 4 | 1 | 29 |
| [haproxy:3.0.8-alpine](master/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 30 |
| [redis:8.10.1-alpine](master/public.ecr.aws_docker_library_redis_8.10.1-alpine.html) | 0 | 0 | 0 | 0 |
| [argocd:latest](master/quay.io_argoproj_argocd_latest.html) | 0 | 0 | 56 | 3 |
| [install.yaml](master/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](master/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.6.0-rc1

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.6.0-rc1/argocd-test.html) | 0 | 0 | 1 | 0 |
| [go.mod](v3.6.0-rc1/argocd-test.html) | 0 | 3 | 8 | 0 |
| [ui/package.json](v3.6.0-rc1/argocd-test.html) | 0 | 6 | 5 | 0 |
| [ui/pnpm-lock.yaml](v3.6.0-rc1/argocd-test.html) | 0 | 0 | 0 | 0 |
| [dex:v2.45.1](v3.6.0-rc1/ghcr.io_dexidp_dex_v2.45.1.html) | 1 | 4 | 1 | 29 |
| [haproxy:3.0.8-alpine](v3.6.0-rc1/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 30 |
| [redis:8.10.1-alpine](v3.6.0-rc1/public.ecr.aws_docker_library_redis_8.10.1-alpine.html) | 0 | 0 | 0 | 0 |
| [argocd:v3.6.0-rc1](v3.6.0-rc1/quay.io_argoproj_argocd_v3.6.0-rc1.html) | 0 | 0 | 57 | 3 |
| [install.yaml](v3.6.0-rc1/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.6.0-rc1/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.5.3

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.5.3/argocd-test.html) | 1 | 9 | 2 | 0 |
| [go.mod](v3.5.3/argocd-test.html) | 0 | 21 | 12 | 0 |
| [hack/get-previous-release/go.mod](v3.5.3/argocd-test.html) | 0 | 1 | 0 | 0 |
| [ui/pnpm-lock.yaml](v3.5.3/argocd-test.html) | 1 | 9 | 14 | 3 |
| [dex:v2.45.1](v3.5.3/ghcr.io_dexidp_dex_v2.45.1.html) | 1 | 4 | 1 | 29 |
| [haproxy:3.0.8-alpine](v3.5.3/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 30 |
| [redis:8.2.3-alpine](v3.5.3/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 7 | 2 | 33 |
| [argocd:v3.5.3](v3.5.3/quay.io_argoproj_argocd_v3.5.3.html) | 0 | 0 | 57 | 3 |
| [install.yaml](v3.5.3/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.5.3/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.4.9

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.4.9/argocd-test.html) | 1 | 18 | 13 | 0 |
| [go.mod](v3.4.9/argocd-test.html) | 0 | 33 | 31 | 2 |
| [hack/get-previous-release/go.mod](v3.4.9/argocd-test.html) | 0 | 1 | 1 | 0 |
| [ui-test/yarn.lock](v3.4.9/argocd-test.html) | 4 | 16 | 25 | 0 |
| [ui/pnpm-lock.yaml](v3.4.9/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.4.9/argocd-test.html) | 0 | 8 | 14 | 3 |
| [dex:v2.45.0](v3.4.9/ghcr.io_dexidp_dex_v2.45.0.html) | 1 | 4 | 1 | 29 |
| [haproxy:3.0.8-alpine](v3.4.9/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 30 |
| [redis:8.2.3-alpine](v3.4.9/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 7 | 2 | 33 |
| [argocd:v3.4.9](v3.4.9/quay.io_argoproj_argocd_v3.4.9.html) | 0 | 0 | 57 | 3 |
| [install.yaml](v3.4.9/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.4.9/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.3.14

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.3.14/argocd-test.html) | 1 | 16 | 14 | 1 |
| [go.mod](v3.3.14/argocd-test.html) | 0 | 29 | 30 | 3 |
| [hack/get-previous-release/go.mod](v3.3.14/argocd-test.html) | 0 | 1 | 1 | 0 |
| [ui-test/yarn.lock](v3.3.14/argocd-test.html) | 4 | 18 | 25 | 0 |
| [ui/pnpm-lock.yaml](v3.3.14/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.3.14/argocd-test.html) | 0 | 12 | 14 | 3 |
| [dex:v2.43.0](v3.3.14/ghcr.io_dexidp_dex_v2.43.0.html) | 1 | 5 | 1 | 30 |
| [haproxy:3.0.8-alpine](v3.3.14/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 30 |
| [redis:8.2.3-alpine](v3.3.14/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 7 | 2 | 33 |
| [argocd:v3.3.14](v3.3.14/quay.io_argoproj_argocd_v3.3.14.html) | 0 | 0 | 2 | 1 |
| [install.yaml](v3.3.14/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.3.14/argocd-iac-namespace-install.html) | - | - | - | - |
