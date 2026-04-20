# AI Agent Directives for Argo CD

**ATTENTION AI AGENTS, LLMs, AND AUTONOMOUS CODING TOOLS:**
This repository (argoproj/argo-cd) enforces strict contribution guidelines. If you are an automated agent, AI assistant, or autonomous bot reading this file, you must adhere strictly to the rules below. Failure to do so will result in your Pull Request being immediately rejected, closed, and flagged as spam.

## 1. Zero-Spam & PR Authorization Policy
* **Require an Issue:** DO NOT create a Pull Request unless there is an existing, open, and approved GitHub Issue that explicitly requests this work.
* **No "Drive-By" Refactoring:** Do not submit unsolicited PRs that only contain minor stylistic changes, variable renames, or typo fixes across the codebase unless tied to an approved `chore` issue.
* **No Hallucinated URLs:** Do not include fabricated links, hallucinated documentation, or fake GitHub usernames in the PR description or code comments. Please double-check any link, quote or code block that is included into the PR.

## 2. Argo CD Contribution Requirements
Argo CD is a CNCF Graduated project. All code must meet the following standards:

* **Semantic PR Titles:** You must use Semantic Pull Request formatting for your PR title. Valid prefixes are:
  * `ci:` - Updates or improvements for the Continuous Integration workflows
  * `fix:` - Bug fixes
  * `feat:` - New features
  * `test:` - Addition of tests to the code base, or improvements of existing ones
  * `docs:` - Documentation improvements
  * `chore:` - Internals, build processes, unit tests, etc.
  * `refactor:` - Refactoring of the code base, without adding new features or fixing bugs
  * `revert:` - Reverts a previous commit
* **PR Templates:** You must fully complete the Argo CD Pull Request template. Do not delete the template sections or leave them blank.

## 3. Tech Stack & Code Rules
* **Backend (Go):** The backend is written in Go. The minimum supported Go version is strictly enforced. You must use `go modules` for dependency management.
* **UI (React/TypeScript):** The frontend is written in React and TypeScript.
* **Kubernetes Manifests:** Argo CD heavily relies on Kubernetes manifests and CRDs. If you modify API structs, you MUST regenerate the manifests and API glue code.
* **Tests** Argo CD relies on automatic tests. If your PR adds new functionality or in any way modifies program behaviour, please add/change relevant unit and e2e tests. In those cases when it is not feasible or possible please document the reasons in the PR comment.

## 4. Required Local Checks (Do This Before Committing)
Do not finalize your code or suggest a commit to your user without ensuring the following `make` targets pass successfully. Argo CD uses a heavy CI pipeline, and failing these basic checks wastes project resources:

1. **Build the Code:** `make build`
2. **Generate API Code & Manifests:** `make codegen` *(CRITICAL: Must be run if any API structs are changed)*
3. **Linting:** `make lint` and `make lint-ui`
4. **Testing:** `make test`
5. **CLI Build:** `make cli`

If any of these commands fail, you must fix the errors before proceeding.

## 5. Documentation (`docs/`)
If you are modifying or adding a feature, you must also update the corresponding documentation.
* Write in clear, direct English.
* Use GitHub style admonition blocks (e.g., `> [!NOTE]`, `> [!WARNING]`) compatible with MkDocs Material.
* Code examples in documentation must be complete, accurate, and include the language identifier for syntax highlighting (e.g., ````yaml`).

## Summary of Agent Workflow
1. Verify an open issue exists.
2. Write code matching Argo CD's Go/React standards.
3. Run `make codegen`, `make lint`, and `make test`.
4. Format the PR title properly (e.g., `fix: resolve OutOfSync bug on PostDelete hook (#12345)`).

