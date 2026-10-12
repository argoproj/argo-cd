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
| [go.mod](master/argocd-test.html) | 0 | 0 | 8 | 0 |
| [ui/package.json](master/argocd-test.html) | 0 | 8 | 6 | 0 |
| [ui/pnpm-lock.yaml](master/argocd-test.html) | 0 | 0 | 0 | 0 |
| [dex:v2.46.0](master/ghcr.io_dexidp_dex_v2.46.0.html) | 0 | 1 | 0 | 0 |
| [haproxy:3.0.8-alpine](master/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 6 | 1 | 40 |
| [redis:8.10.2-alpine](master/public.ecr.aws_docker_library_redis_8.10.2-alpine.html) | 0 | 1 | 0 | 0 |
| [argocd:latest](master/quay.io_argoproj_argocd_latest.html) | 0 | 0 | 31 | 3 |
| [install.yaml](master/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](master/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.6.0-rc2

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.6.0-rc2/argocd-test.html) | 0 | 0 | 1 | 0 |
| [go.mod](v3.6.0-rc2/argocd-test.html) | 0 | 0 | 8 | 0 |
| [ui/package.json](v3.6.0-rc2/argocd-test.html) | 0 | 9 | 6 | 0 |
| [ui/pnpm-lock.yaml](v3.6.0-rc2/argocd-test.html) | 0 | 0 | 0 | 0 |
| [dex:v2.45.1](v3.6.0-rc2/ghcr.io_dexidp_dex_v2.45.1.html) | 1 | 5 | 1 | 29 |
| [haproxy:3.0.8-alpine](v3.6.0-rc2/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 6 | 1 | 40 |
| [redis:8.10.1-alpine](v3.6.0-rc2/public.ecr.aws_docker_library_redis_8.10.1-alpine.html) | 0 | 1 | 0 | 0 |
| [argocd:v3.6.0-rc2](v3.6.0-rc2/quay.io_argoproj_argocd_v3.6.0-rc2.html) | 0 | 0 | 31 | 3 |
| [install.yaml](v3.6.0-rc2/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.6.0-rc2/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.5.4

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.5.4/argocd-test.html) | 1 | 9 | 2 | 0 |
| [go.mod](v3.5.4/argocd-test.html) | 0 | 21 | 12 | 0 |
| [hack/get-previous-release/go.mod](v3.5.4/argocd-test.html) | 0 | 1 | 0 | 0 |
| [ui/pnpm-lock.yaml](v3.5.4/argocd-test.html) | 1 | 12 | 16 | 3 |
| [dex:v2.45.1](v3.5.4/ghcr.io_dexidp_dex_v2.45.1.html) | 1 | 5 | 1 | 29 |
| [haproxy:3.0.8-alpine](v3.5.4/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 6 | 1 | 40 |
| [redis:8.2.3-alpine](v3.5.4/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 8 | 2 | 33 |
| [argocd:v3.5.4](v3.5.4/quay.io_argoproj_argocd_v3.5.4.html) | 0 | 0 | 31 | 3 |
| [install.yaml](v3.5.4/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.5.4/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.4.10

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.4.10/argocd-test.html) | 1 | 18 | 13 | 0 |
| [go.mod](v3.4.10/argocd-test.html) | 0 | 33 | 28 | 2 |
| [hack/get-previous-release/go.mod](v3.4.10/argocd-test.html) | 0 | 1 | 1 | 0 |
| [ui-test/yarn.lock](v3.4.10/argocd-test.html) | 4 | 20 | 33 | 0 |
| [ui/pnpm-lock.yaml](v3.4.10/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.4.10/argocd-test.html) | 0 | 9 | 16 | 3 |
| [dex:v2.45.0](v3.4.10/ghcr.io_dexidp_dex_v2.45.0.html) | 1 | 5 | 1 | 29 |
| [haproxy:3.0.8-alpine](v3.4.10/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 6 | 1 | 40 |
| [redis:8.2.3-alpine](v3.4.10/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 8 | 2 | 33 |
| [argocd:v3.4.10](v3.4.10/quay.io_argoproj_argocd_v3.4.10.html) | 0 | 0 | 31 | 3 |
| [install.yaml](v3.4.10/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.4.10/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.3.15

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.3.15/argocd-test.html) | 1 | 16 | 14 | 1 |
| [go.mod](v3.3.15/argocd-test.html) | 0 | 30 | 27 | 3 |
| [hack/get-previous-release/go.mod](v3.3.15/argocd-test.html) | 0 | 1 | 1 | 0 |
| [ui-test/yarn.lock](v3.3.15/argocd-test.html) | 4 | 19 | 33 | 0 |
| [ui/pnpm-lock.yaml](v3.3.15/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.3.15/argocd-test.html) | 0 | 12 | 15 | 3 |
| [dex:v2.43.0](v3.3.15/ghcr.io_dexidp_dex_v2.43.0.html) | 1 | 6 | 1 | 40 |
| [haproxy:3.0.8-alpine](v3.3.15/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 6 | 1 | 40 |
| [redis:8.2.3-alpine](v3.3.15/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 8 | 2 | 33 |
| [argocd:v3.3.15](v3.3.15/quay.io_argoproj_argocd_v3.3.15.html) | 0 | 0 | 1 | 0 |
| [install.yaml](v3.3.15/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.3.15/argocd-iac-namespace-install.html) | - | - | - | - |
