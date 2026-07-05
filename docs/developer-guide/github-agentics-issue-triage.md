# GitHub Agentics Workflows

## Overview

GitHub Agentics offers multiple AI GitHub workflows for repo maintainers.

Argo CD maintainers use [GitHub Agentics automatic issue triage](https://github.com/githubnext/agentics/blob/main/docs/issue-triage.md).

This particular workflow is performing auto-triage of issues, by commenting on them, closing duplicates and labeling the issues.

Github Agentics is essentially a GitHub workflow, which is created and updated by a dedicated CLI, based on some configuration files that the Argo CD maintainers interact with.

The initial configuration was performed by running `gh aw add-wizard githubnext/agentics/issue-triage` locally, this was a one-time step. 

## Configuration

A dedicated GitHub `gh aw` CLI needs to be installed locally, by running `make install-gh-aw-local`. The CLI is also installed as part of `make install-tools-local`. 

- `.github/workflows/issue-triage.md` - the main file with which the maintainers can interact to configure the workflow. It contains both a prompt for the agent and a configuration section. The file is pre-created upon the initial installation of the wizard and then it can be configured further. Upon performing changes in this file, it is required to run `gh aw compile` or `make codegen-local`. 
- `.github/workflows/aw.json` - additional configuration file. Upon performing changes in this file, it is required to run `gh aw compile` or `make codegen-local`.

## Auto generated workflow files
- `.github/workflows/issue-triage.lock.yml` - the GitHub CI workflow itself, auto-generated based on the `issue-triage.md` and `aw.json` files. 
- `.github/aw/actions-lock.json` - pinned version of the relevant GitHub actions, auto-generated based on the `issue-triage.md` and `aw.json` files.
- `.github/workflows/agentics-maintenance.yml` - a generic workflow that performs cleanup for multiple Github Agentics workflows types, not specific to triage. The current design of this workflow violates the principle of least privilege, requiring content and pull requests write permissions. This workflow is disabled while the required fine-grained permissions are [in discussion](https://github.com/github/gh-aw/issues/42779).  

## AI credits usage
Github Agentics can work with multiple AI models. It requires an AI token, which can be configured explicitly as a repo secret or implicitly by using GitHub organization Copilot Premium seat. Argo CD repo uses the latter.