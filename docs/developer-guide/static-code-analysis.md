# Code Quality and Security Scanning

We use the following code quality and security scanning tools:

* `golangci-lint` and `eslint` for compile time linting
* [CodeQL](https://codeql.github.com/) - for semantic code analysis
* [codecov.io](https://codecov.io/gh/argoproj/argo-cd) - for code coverage
* [snyk.io](https://app.snyk.io/org/argoproj/projects) - for image scanning
* [sonarcloud.io](https://sonarcloud.io/organizations/argoproj/projects) - for code scans and security alerts

These are at least run daily or on each pull request.
