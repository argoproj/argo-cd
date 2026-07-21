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
| [go.mod](master/argocd-test.html) | 0 | 0 | 1 | 0 |
| [ui/pnpm-lock.yaml](master/argocd-test.html) | 1 | 2 | 11 | 1 |
| [dex:v2.45.0](master/ghcr.io_dexidp_dex_v2.45.0.html) | 1 | 4 | 1 | 19 |
| [haproxy:3.0.8-alpine](master/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 17 |
| [redis:8.2.3-alpine](master/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 4 | 1 | 19 |
| [argocd:latest](master/quay.io_argoproj_argocd_latest.html) | 0 | 0 | 16 | 7 |
| [install.yaml](master/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](master/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.5.0-rc2

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.5.0-rc2/argocd-test.html) | 1 | 3 | 1 | 0 |
| [go.mod](v3.5.0-rc2/argocd-test.html) | 0 | 2 | 2 | 0 |
| [ui/pnpm-lock.yaml](v3.5.0-rc2/argocd-test.html) | 1 | 2 | 11 | 1 |
| [dex:v2.45.0](v3.5.0-rc2/ghcr.io_dexidp_dex_v2.45.0.html) | 1 | 4 | 1 | 19 |
| [haproxy:3.0.8-alpine](v3.5.0-rc2/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 17 |
| [redis:8.2.3-alpine](v3.5.0-rc2/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 4 | 1 | 19 |
| [argocd:v3.5.0-rc2](v3.5.0-rc2/quay.io_argoproj_argocd_v3.5.0-rc2.html) | 0 | 0 | 17 | 7 |
| [install.yaml](v3.5.0-rc2/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.5.0-rc2/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.4.4

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.4.4/argocd-test.html) | 1 | 12 | 12 | 0 |
| [go.mod](v3.4.4/argocd-test.html) | 1 | 22 | 33 | 2 |
| [hack/get-previous-release/go.mod](v3.4.4/argocd-test.html) | 0 | 0 | 1 | 0 |
| [ui-test/yarn.lock](v3.4.4/argocd-test.html) | 4 | 15 | 15 | 0 |
| [ui/pnpm-lock.yaml](v3.4.4/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.4.4/argocd-test.html) | 0 | 10 | 17 | 3 |
| [dex:v2.45.0](v3.4.4/ghcr.io_dexidp_dex_v2.45.0.html) | 1 | 4 | 1 | 19 |
| [haproxy:3.0.8-alpine](v3.4.4/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 17 |
| [redis:8.2.3-alpine](v3.4.4/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 4 | 1 | 19 |
| [argocd:v3.4.4](v3.4.4/quay.io_argoproj_argocd_v3.4.4.html) | 0 | 0 | 70 | 17 |
| [install.yaml](v3.4.4/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.4.4/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.3.12

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.3.12/argocd-test.html) | 1 | 11 | 13 | 1 |
| [go.mod](v3.3.12/argocd-test.html) | 1 | 19 | 32 | 3 |
| [hack/get-previous-release/go.mod](v3.3.12/argocd-test.html) | 0 | 0 | 1 | 0 |
| [ui-test/yarn.lock](v3.3.12/argocd-test.html) | 4 | 17 | 16 | 0 |
| [ui/pnpm-lock.yaml](v3.3.12/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.3.12/argocd-test.html) | 0 | 10 | 17 | 3 |
| [dex:v2.43.0](v3.3.12/ghcr.io_dexidp_dex_v2.43.0.html) | 1 | 5 | 1 | 17 |
| [haproxy:3.0.8-alpine](v3.3.12/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 17 |
| [redis:8.2.3-alpine](v3.3.12/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 4 | 1 | 19 |
| [argocd:v3.3.12](v3.3.12/quay.io_argoproj_argocd_v3.3.12.html) | 0 | 0 | 70 | 19 |
| [install.yaml](v3.3.12/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.3.12/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.2.12

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [go.mod](v3.2.12/argocd-test.html) | 2 | 26 | 30 | 3 |
| [hack/get-previous-release/go.mod](v3.2.12/argocd-test.html) | 0 | 0 | 1 | 0 |
| [ui-test/yarn.lock](v3.2.12/argocd-test.html) | 4 | 17 | 18 | 0 |
| [ui/pnpm-lock.yaml](v3.2.12/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.2.12/argocd-test.html) | 0 | 10 | 26 | 4 |
| [dex:v2.43.0](v3.2.12/ghcr.io_dexidp_dex_v2.43.0.html) | 1 | 5 | 1 | 17 |
| [haproxy:3.0.8-alpine](v3.2.12/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 5 | 1 | 17 |
| [redis:8.2.2-alpine](v3.2.12/public.ecr.aws_docker_library_redis_8.2.2-alpine.html) | 1 | 5 | 1 | 32 |
| [argocd:v3.2.12](v3.2.12/quay.io_argoproj_argocd_v3.2.12.html) | 0 | 0 | 0 | 0 |
| [install.yaml](v3.2.12/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.2.12/argocd-iac-namespace-install.html) | - | - | - | - |
