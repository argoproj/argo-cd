---
name: run-test
description: Run Go tests by function name, package path, or file pattern. Usage /run-test TestFunctionName or /run-test ./pkg/apis/...
args: "<test-name-or-package>"
---

Run Go tests in the Argo CD codebase. Determine what the user wants to test from the argument:

## Argument Patterns

1. **Test function name** (starts with `Test`): Run that specific test
   ```bash
   make TEST_FLAGS="-run <function-name>" test-local
   ```

2. **Package path** (contains `/` or `./`): Run all tests in that package
   ```bash
   make TEST_MODULE=github.com/argoproj/argo-cd/v3/<path> test-local
   ```
   If the argument starts with `./`, prepend the module path: `github.com/argoproj/argo-cd/v3/`

3. **File pattern** (ends with `_test.go`): Find the package and run it
   - Use Glob to find the file
   - Extract the package path from its directory
   - Run with TEST_MODULE

4. **"race"** keyword included: Add race detection
   ```bash
   make TEST_FLAGS="-run <name>" test-race-local
   ```

5. **"e2e"** keyword included: Use E2E test target
   ```bash
   make TEST_FLAGS="-run <name>" test-e2e-local
   ```

6. **"verbose"** keyword included: Add verbosity
   ```bash
   ARGOCD_TEST_VERBOSE=1 make TEST_FLAGS="-run <name>" test-local
   ```

## Environment Variables

- `ARGOCD_TEST_PARALLELISM=N` — Control parallel test count
- `ARGOCD_TEST_VERBOSE=1` — Verbose output
- `RERUN_FAILS=N` — Retry failed tests N times

## Examples

- `/run-test TestCreateApp` — Run single test function
- `/run-test ./util/git/...` — Run all tests in util/git
- `/run-test TestSync race` — Run TestSync with race detection
- `/run-test TestE2E e2e` — Run E2E test
