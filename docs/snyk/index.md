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
| [ui/package.json](master/argocd-test.html) | 0 | 9 | 8 | 0 |
| [ui/pnpm-lock.yaml](master/argocd-test.html) | 0 | 0 | 0 | 0 |
| [dex:v2.46.1](master/ghcr.io_dexidp_dex_v2.46.1.html) | 0 | 1 | 0 | 1 |
| [haproxy:3.0.8-alpine](master/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 8 | 4 | 43 |
| [redis:8.10.3-alpine](master/public.ecr.aws_docker_library_redis_8.10.3-alpine.html) | 0 | 0 | 0 | 0 |
| [argocd:latest](master/quay.io_argoproj_argocd_latest.html) | 0 | 0 | 46 | 3 |
| [install.yaml](master/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](master/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.6.0-rc4

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.6.0-rc4/argocd-test.html) | 0 | 1 | 1 | 0 |
| [go.mod](v3.6.0-rc4/argocd-test.html) | 0 | 3 | 9 | 0 |
| [ui/package.json](v3.6.0-rc4/argocd-test.html) | 0 | 9 | 8 | 0 |
| [ui/pnpm-lock.yaml](v3.6.0-rc4/argocd-test.html) | 0 | 0 | 0 | 0 |
| [dex:v2.46.0](v3.6.0-rc4/ghcr.io_dexidp_dex_v2.46.0.html) | 0 | 1 | 0 | 1 |
| [haproxy:3.0.8-alpine](v3.6.0-rc4/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 8 | 4 | 43 |
| [redis:8.10.1-alpine](v3.6.0-rc4/public.ecr.aws_docker_library_redis_8.10.1-alpine.html) | 0 | 1 | 0 | 1 |
| [argocd:v3.6.0-rc4](v3.6.0-rc4/quay.io_argoproj_argocd_v3.6.0-rc4.html) | 0 | 1 | 47 | 3 |
| [install.yaml](v3.6.0-rc4/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.6.0-rc4/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.5.6

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.5.6/argocd-test.html) | 1 | 10 | 2 | 0 |
| [go.mod](v3.5.6/argocd-test.html) | 0 | 13 | 14 | 3 |
| [hack/get-previous-release/go.mod](v3.5.6/argocd-test.html) | 0 | 1 | 0 | 0 |
| [ui/pnpm-lock.yaml](v3.5.6/argocd-test.html) | 3 | 14 | 12 | 1 |
| [dex:v2.45.1](v3.5.6/ghcr.io_dexidp_dex_v2.45.1.html) | 1 | 5 | 1 | 30 |
| [haproxy:3.0.8-alpine](v3.5.6/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 8 | 4 | 43 |
| [redis:8.2.3-alpine](v3.5.6/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 8 | 2 | 34 |
| [argocd:v3.5.6](v3.5.6/quay.io_argoproj_argocd_v3.5.6.html) | 0 | 1 | 47 | 3 |
| [install.yaml](v3.5.6/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.5.6/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.4.12

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.4.12/argocd-test.html) | 1 | 20 | 13 | 0 |
| [go.mod](v3.4.12/argocd-test.html) | 0 | 26 | 25 | 4 |
| [hack/get-previous-release/go.mod](v3.4.12/argocd-test.html) | 0 | 1 | 1 | 0 |
| [ui-test/yarn.lock](v3.4.12/argocd-test.html) | 4 | 20 | 33 | 0 |
| [ui/pnpm-lock.yaml](v3.4.12/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.4.12/argocd-test.html) | 2 | 12 | 15 | 2 |
| [dex:v2.45.0](v3.4.12/ghcr.io_dexidp_dex_v2.45.0.html) | 1 | 5 | 1 | 30 |
| [haproxy:3.0.8-alpine](v3.4.12/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 8 | 4 | 43 |
| [redis:8.2.3-alpine](v3.4.12/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 8 | 2 | 34 |
| [argocd:v3.4.12](v3.4.12/quay.io_argoproj_argocd_v3.4.12.html) | 0 | 1 | 47 | 3 |
| [install.yaml](v3.4.12/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.4.12/argocd-iac-namespace-install.html) | - | - | - | - |

### v3.3.18

|    | Critical | High | Medium | Low |
|---:|:--------:|:----:|:------:|:---:|
| [gitops-engine/go.mod](v3.3.18/argocd-test.html) | 1 | 18 | 14 | 1 |
| [go.mod](v3.3.18/argocd-test.html) | 0 | 28 | 23 | 5 |
| [hack/get-previous-release/go.mod](v3.3.18/argocd-test.html) | 0 | 1 | 1 | 0 |
| [ui-test/yarn.lock](v3.3.18/argocd-test.html) | 4 | 19 | 33 | 0 |
| [ui/pnpm-lock.yaml](v3.3.18/argocd-test.html) | 0 | 0 | 0 | 0 |
| [ui/yarn.lock](v3.3.18/argocd-test.html) | 2 | 15 | 13 | 2 |
| [dex:v2.43.0](v3.3.18/ghcr.io_dexidp_dex_v2.43.0.html) | 1 | 6 | 1 | 41 |
| [haproxy:3.0.8-alpine](v3.3.18/public.ecr.aws_docker_library_haproxy_3.0.8-alpine.html) | 1 | 8 | 4 | 43 |
| [redis:8.2.3-alpine](v3.3.18/public.ecr.aws_docker_library_redis_8.2.3-alpine.html) | 1 | 8 | 2 | 34 |
| [argocd:v3.3.18](v3.3.18/quay.io_argoproj_argocd_v3.3.18.html) | 0 | 0 | 1 | 0 |
| [install.yaml](v3.3.18/argocd-iac-install.html) | - | - | - | - |
| [namespace-install.yaml](v3.3.18/argocd-iac-namespace-install.html) | - | - | - | - |
