# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Argo CD is a declarative GitOps continuous deployment tool for Kubernetes. Version 3.4.0, written in Go 1.26.0 with a React/TypeScript UI. Module path: `github.com/argoproj/argo-cd/v3`.

## Build & Development Commands

Most targets exist in both Docker (`make <target>`) and local (`make <target>-local`) variants. Prefer local variants for faster iteration.

### Build
```bash
make build-local          # Build all Go code
make cli-local            # Build argocd CLI binary
make build-ui             # Build React UI (or: cd ui && yarn build)
```

### Test
```bash
make test-local                                        # All unit tests
make TEST_MODULE=github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1 test-local  # Specific module
make TEST_FLAGS="-run TestFunctionName" test-local     # Single test function
ARGOCD_TEST_PARALLELISM=4 make test-local              # Control parallelism
make test-race-local                                   # With race detection
```

### E2E Tests
```bash
make start-e2e-local      # Terminal 1: start all services
make test-e2e-local       # Terminal 2: run E2E suite
make TEST_FLAGS="-run TestName" test-e2e-local  # Single E2E test
```

### Lint
```bash
make lint-local           # Go linting (golangci-lint)
make lint-ui-local        # UI linting (cd ui && yarn lint)
```

### Code Generation
```bash
make codegen-local        # Full codegen (protobuf, clients, mocks, manifests, swagger)
make codegen-local-fast   # Without mod-vendor step
make protogen-fast        # Protobuf only
make clientgen            # K8s client codegen only
make mockgen              # Mock generation only (uses .mockery.yaml)
```

### Full Validation Cycle
```bash
make pre-commit-local     # codegen + build + lint + test
```

### Tool Installation
```bash
make install-tools-local  # Install all required dev tools (kustomize, helm, gotestsum, etc.)
```

### Start Local Dev Environment
```bash
make start-local          # Starts all components via goreman (see Procfile)
# UI at localhost:4000, API at localhost:8080
```

## Architecture

### Component Communication
```
CLI/UI → argocd-server (gRPC + HTTP/grpc-gateway, :8080)
              ↓
         argocd-repo-server (gRPC, :8081) → Git/Helm/Kustomize
              ↓
         argocd-application-controller → GitOps Engine → Target Clusters
              ↓
         argocd-redis (:6379) — caching/sessions
```

### Key Directories
- **cmd/** — Single binary entry point (`main.go`), dispatches via `ARGOCD_BINARY_NAME` env var
- **server/** — API server with gRPC services (Application, Repository, Project, Cluster, Session, Settings, etc.)
- **controller/** — Application reconciliation controller with work queues (refresh, operation, hydration)
- **reposerver/** — Manifest generation from Git/Helm/Kustomize/Jsonnet sources
- **applicationset/** — ApplicationSet templating controller with generators (List, Cluster, Git, Matrix, Merge, SCM)
- **gitops-engine/** — Core diff/sync/health engine (separate Go module at `gitops-engine/`)
- **pkg/apis/application/v1alpha1/** — CRD type definitions (Application, AppProject, ApplicationSet) with protobuf
- **pkg/apiclient/** — gRPC client library for all server services
- **pkg/client/** — Auto-generated Kubernetes clientsets
- **resource_customizations/** — Lua scripts for per-resource health checks and custom actions, organized by `group/Kind/`
- **util/** — 60+ utility packages (db, cache, git, helm, kustomize, oidc, rbac, session, settings, etc.)
- **hack/** — Dev scripts (test runner, tool installer, codegen helpers)
- **ui/** — React/TypeScript frontend
- **test/** — E2E test suite and test fixtures
- **manifests/** — Kubernetes installation manifests

### Key Patterns
- **Single binary**: All components compile into one binary; `ARGOCD_BINARY_NAME` selects which to run
- **gRPC + grpc-gateway**: Services defined in `.proto` files, HTTP auto-translated from gRPC
- **Informer pattern**: Kubernetes shared informers for efficient CRD watching
- **Work queue pattern**: Rate-limited queues for async reconciliation (appRefreshQueue, appOperationQueue, etc.)
- **Lua customizations**: Runtime-extensible health checks and actions in `resource_customizations/` without recompilation
- **Controller sharding**: Multiple controller instances distribute Application workload

### Storage
- **etcd** (via K8s API): Application, AppProject, ApplicationSet CRDs
- **K8s Secrets**: Cluster credentials (`argocd.argoproj.io/secret-type=cluster`), repo credentials (`secret-type=repository`)
- **ConfigMaps**: Settings (`argocd-cm`), SSH known hosts, TLS certs, GPG keys
- **Redis**: Session state, manifest cache, user state

## Code Conventions

### Go Import Aliases (enforced by importas linter)
```go
import (
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    rbacv1 "k8s.io/api/rbac/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
    informersv1 "k8s.io/client-go/informers/core/v1"
    jwtgo "github.com/golang-jwt/jwt/v5"
    stderrors "errors"
    utilio "github.com/argoproj/argo-cd/v3/util/io"
)
```

### Blocked Dependencies (enforced by gomodguard)
- Use `github.com/golang-jwt/jwt/v5` not v4
- Use `dario.cat/mergo` not `github.com/imdario/mergo`
- Use `errors` not `github.com/pkg/errors`

### Formatting
- **gofumpt** + **goimports** (with local prefix `github.com/argoproj/argo-cd/v3`)
- Var naming: `ID` (not `Id`), `VM` (not `Vm`), uppercase constants allowed

### Linting Highlights
- `errorlint`: Wrap errors properly, use `errors.Is`/`errors.As`
- `noctx`: Pass `context.Context` to HTTP requests
- `perfsprint`: Prefer `strconv` over `fmt.Sprintf` for simple conversions
- `tparallel`/`thelper`: Use `t.Parallel()` and `t.Helper()` properly in tests
- `testifylint`: Consistent testify assertion usage
- `revive/early-return`: Prefer early returns, avoid deep nesting

### Proto/gRPC
- Service definitions in `server/<service>/<service>.proto`
- Run `make protogen-fast` after modifying `.proto` files
- Generated code goes alongside proto files

### Resource Customizations
When adding health checks or actions for a Kubernetes resource:
1. Create `resource_customizations/<group>/<Kind>/`
2. Add `health.lua` for health assessment
3. Add `actions/` directory with `action.lua` and `discovery.lua` for custom actions
