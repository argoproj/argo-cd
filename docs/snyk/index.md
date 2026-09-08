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
| [gitops-engine/go.mod](master/argocd-test.html) | 0 | 0 | 2 | 0 |
| [go.mod](master/argocd-test.html) | 0 | 5 | 8 | 0 |
| [ui/pnpm-lock.yaml](master/argocd-test.html) | 0 | 5 | 5 | 0 |
| [dex:v2.45.1](master/ghcr.io_dexidp_dex_v2.45.1.html) | 1 | 4 | 1 | 29 |
| [haproxy:3.0.8-alpine](master/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 17 |
| [redis:8.10.1-alpine](master/public.ecr.aws_docker_library_redis_8.10.1-alpine.html) | 0 | 3 | 0 | 13 |
| [argocd:latest](master/quay.io_argoproj_argocd_latest.html) | 0 | 0 | 50 | 3 |
| [install.yaml](master/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](master/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.5.2

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.5.2/argocd-test.html) | 1 | 9 | 2 | 0 |
| [go.mod](v3.5.2/argocd-test.html) | 0 | 20 | 12 | 0 |
| [hack/get-previous-release/go.mod](v3.5.2/argocd-test.html) | 0 | 1 | 0 | 0 |
| [ui/pnpm-lock.yaml](v3.5.2/argocd-test.html) | 1 | 8 | 14 | 3 |
| [dex:v2.45.1](v3.5.2/ghcr.io_dexidp_dex_v2.45.1.html) | 1 | 4 | 1 | 29 |
| [haproxy:3.0.8-alpine](v3.5.2/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 17 |
| [redis:8.2.3-alpine](v3.5.2/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 7 | 2 | 33 |
| [argocd:v3.5.2](v3.5.2/quay.io_argoproj_argocd_v3.5.2.html) | 0 | 0 | 66 | 9 |
| [install.yaml](v3.5.2/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.5.2/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.4.8

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.4.8/argocd-test.html) | 1 | 18 | 13 | 0 |
| [go.mod](v3.4.8/argocd-test.html) | 0 | 32 | 31 | 2 |
| [hack/get-previous-release/go.mod](v3.4.8/argocd-test.html) | 0 | 1 | 1 | 0 |
| [ui-test/yarn.lock](v3.4.8/argocd-test.html) | 4 | 15 | 25 | 0 |
| [ui/pnpm-lock.yaml](v3.4.8/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.4.8/argocd-test.html) | 0 | 7 | 14 | 3 |
| [dex:v2.45.0](v3.4.8/ghcr.io_dexidp_dex_v2.45.0.html) | 1 | 4 | 1 | 29 |
| [haproxy:3.0.8-alpine](v3.4.8/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 17 |
| [redis:8.2.3-alpine](v3.4.8/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 7 | 2 | 33 |
| [argocd:v3.4.8](v3.4.8/quay.io_argoproj_argocd_v3.4.8.html) | 0 | 0 | 66 | 9 |
| [install.yaml](v3.4.8/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.4.8/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.3.14

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.3.14/argocd-test.html) | 1 | 16 | 14 | 1 |
| [go.mod](v3.3.14/argocd-test.html) | 0 | 28 | 30 | 3 |
| [hack/get-previous-release/go.mod](v3.3.14/argocd-test.html) | 0 | 1 | 1 | 0 |
| [ui-test/yarn.lock](v3.3.14/argocd-test.html) | 4 | 17 | 25 | 0 |
| [ui/pnpm-lock.yaml](v3.3.14/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.3.14/argocd-test.html) | 0 | 11 | 14 | 3 |
| [dex:v2.43.0](v3.3.14/ghcr.io_dexidp_dex_v2.43.0.html) | 1 | 5 | 1 | 17 |
| [haproxy:3.0.8-alpine](v3.3.14/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 17 |
| [redis:8.2.3-alpine](v3.3.14/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 7 | 2 | 33 |
| [argocd:v3.3.14](v3.3.14/quay.io_argoproj_argocd_v3.3.14.html) | 0 | 0 | 2 | 1 |
| [install.yaml](v3.3.14/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.3.14/argocd-iac-namespace-install.html) | - | - | - | - |
