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
| [gitops-engine/go.mod](master/argocd-test.html) | 0 | 1 | 1 | 0 |
| [go.mod](master/argocd-test.html) | 0 | 2 | 9 | 0 |
| [ui/package.json](master/argocd-test.html) | 0 | 8 | 8 | 0 |
| [ui/pnpm-lock.yaml](master/argocd-test.html) | 0 | 0 | 0 | 0 |
| [dex:v2.46.1](master/ghcr.io_dexidp_dex_v2.46.1.html) | 0 | 1 | 0 | 7 |
| [haproxy:3.0.8-alpine](master/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 8 | 4 | 44 |
| [redis:8.10.3-alpine](master/public.ecr.aws_docker_library_redis_8.10.3-alpine.html) | 0 | 0 | 0 | 1 |
| [argocd:latest](master/quay.io_argoproj_argocd_latest.html) | 0 | 0 | 28 | 3 |
| [install.yaml](master/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](master/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.6.2

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.6.2/argocd-test.html) | 0 | 1 | 4 | 0 |
| [go.mod](v3.6.2/argocd-test.html) | 0 | 2 | 13 | 0 |
| [ui/package.json](v3.6.2/argocd-test.html) | 0 | 9 | 8 | 0 |
| [ui/pnpm-lock.yaml](v3.6.2/argocd-test.html) | 0 | 0 | 0 | 0 |
| [dex:v2.46.0](v3.6.2/ghcr.io_dexidp_dex_v2.46.0.html) | 0 | 1 | 0 | 7 |
| [haproxy:3.0.8-alpine](v3.6.2/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 8 | 4 | 44 |
| [redis:8.10.1-alpine](v3.6.2/public.ecr.aws_docker_library_redis_8.10.1-alpine.html) | 0 | 1 | 0 | 2 |
| [argocd:v3.6.2](v3.6.2/quay.io_argoproj_argocd_v3.6.2.html) | 0 | 0 | 28 | 3 |
| [install.yaml](v3.6.2/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.6.2/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.5.8

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.5.8/argocd-test.html) | 1 | 0 | 1 | 0 |
| [go.mod](v3.5.8/argocd-test.html) | 0 | 2 | 13 | 0 |
| [ui/pnpm-lock.yaml](v3.5.8/argocd-test.html) | 3 | 14 | 13 | 1 |
| [dex:v2.45.1](v3.5.8/ghcr.io_dexidp_dex_v2.45.1.html) | 1 | 5 | 1 | 31 |
| [haproxy:3.0.8-alpine](v3.5.8/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 8 | 4 | 44 |
| [redis:8.2.3-alpine](v3.5.8/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 8 | 2 | 35 |
| [argocd:v3.5.8](v3.5.8/quay.io_argoproj_argocd_v3.5.8.html) | 0 | 0 | 28 | 3 |
| [install.yaml](v3.5.8/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.5.8/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.4.14

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.4.14/argocd-test.html) | 1 | 3 | 8 | 0 |
| [go.mod](v3.4.14/argocd-test.html) | 0 | 6 | 15 | 3 |
| [ui-test/yarn.lock](v3.4.14/argocd-test.html) | 4 | 21 | 34 | 0 |
| [ui/pnpm-lock.yaml](v3.4.14/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.4.14/argocd-test.html) | 2 | 12 | 17 | 2 |
| [dex:v2.45.0](v3.4.14/ghcr.io_dexidp_dex_v2.45.0.html) | 1 | 5 | 1 | 31 |
| [haproxy:3.0.8-alpine](v3.4.14/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 8 | 4 | 44 |
| [redis:8.2.3-alpine](v3.4.14/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 8 | 2 | 35 |
| [argocd:v3.4.14](v3.4.14/quay.io_argoproj_argocd_v3.4.14.html) | 0 | 0 | 28 | 3 |
| [install.yaml](v3.4.14/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.4.14/argocd-iac-namespace-install.html) | - | - | - | - |
