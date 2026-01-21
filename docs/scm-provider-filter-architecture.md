# SCM Provider Filter Architecture

## Overview

This document explains how SCM Provider filtering works in ArgoCD ApplicationSets. The filtering system uses a two-phase approach to efficiently filter repositories and branches based on various criteria.

## Two-Phase Filtering Process

The filtering system separates repository filtering from branch filtering to optimize performance and minimize API calls:

```
┌─────────────────────────────────────────────────────────────┐
│                    All Repositories                          │
│              (from GitHub/GitLab/Bitbucket)                  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
        ┌─────────────────────────────────────────┐
        │         PHASE 1: Repo Filtering         │
        │                                          │
        │  • Repository name matching              │
        │  • Label/topic matching                  │
        │                                          │
        │  (Fast - no branch fetching required)   │
        └─────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  Filtered Repos  │
                    │   (Subset only)  │
                    └─────────────────┘
                              │
                              ▼
        ┌─────────────────────────────────────────┐
        │        PHASE 2: Branch Filtering        │
        │                                          │
        │  • Branch name matching                  │
        │  • Path existence checks                 │
        │  • Path non-existence checks             │
        │                                          │
        │  (Slower - requires API calls)           │
        └─────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  Final Results   │
                    └─────────────────┘
```

### Why Two Phases?

**Phase 1: Repository Filtering**
- Evaluates conditions that don't require branch information
- Filters out repositories early to reduce processing
- Fast operations using only repository metadata

**Phase 2: Branch Filtering**
- Only processes repositories that passed Phase 1
- Performs expensive operations (path checks via API)
- Reduces total number of API calls significantly

## Filter Condition Types

### Repo-Level Conditions (Evaluated in Phase 1)

These conditions use only repository metadata:

| Condition | Description | Example |
|-----------|-------------|---------|
| **Repository Match** | Regex pattern for repository name | Match repos starting with "eks-" |
| **Label Match** | Regex pattern for repository labels/topics | Match repos labeled "production" |

**Characteristics:**
- ✅ Fast evaluation
- ✅ No API calls required
- ✅ Uses only cached metadata

### Branch-Level Conditions (Evaluated in Phase 2)

These conditions require branch information and may need API calls:

| Condition | Description | Example |
|-----------|-------------|---------|
| **Branch Match** | Regex pattern for branch name | Match only "main" or "master" branches |
| **Paths Exist** | Verify specified paths exist | Require "kustomization.yaml" to exist |
| **Paths Don't Exist** | Verify specified paths DON'T exist | Exclude repos with ".archived" file |

**Characteristics:**
- ⚠️ Slower evaluation
- ⚠️ Requires API calls to SCM provider
- ⚠️ May be rate-limited

## Filter Categorization Process

When filters are processed, the system examines each filter and determines which phase(s) it should be evaluated in:

```
┌─────────────────────────────────┐
│         Filter Definition        │
└─────────────────────────────────┘
                │
                ▼
┌───────────────────────────────────────────┐
│    Inspect Conditions in Filter          │
│                                           │
│  Has repositoryMatch?  ──┐                │
│  Has labelMatch?        ─┤                │
│                          │                │
│  Has branchMatch?       ─┼──┐             │
│  Has pathsExist?        ─┤  │             │
│  Has pathsDoNotExist?   ─┘  │             │
└───────────────────────────────────────────┘
                │                 │
                ▼                 ▼
    ┌─────────────────┐  ┌─────────────────┐
    │  Add to Repo     │  │  Add to Branch   │
    │  Filter Group    │  │  Filter Group    │
    └─────────────────┘  └─────────────────┘
```

### Mixed Filters

A filter can contain both repo-level AND branch-level conditions:

```
Filter Example:
├─ repositoryMatch: "^eks-"        (Repo-level)
└─ pathsExist: ["config/app.yaml"] (Branch-level)

Categorization Result:
├─ Added to Repo Filter Group      ✓
└─ Added to Branch Filter Group    ✓

Evaluation:
├─ Phase 1: Check repositoryMatch  ✓
└─ Phase 2: Check pathsExist       ✓
```

This ensures ALL conditions in the filter are evaluated across both phases.

## Complete Filtering Flow

### High-Level Process

```
┌──────────────────────┐
│  1. Compile Filters  │
│                      │
│  Convert YAML to     │
│  regex patterns      │
└──────────────────────┘
            │
            ▼
┌──────────────────────┐
│  2. Categorize       │
│                      │
│  Group filters by    │
│  evaluation phase    │
└──────────────────────┘
            │
            ▼
┌──────────────────────┐
│  3. Fetch All Repos  │
│                      │
│  Get repository list │
│  from SCM provider   │
└──────────────────────┘
            │
            ▼
┌──────────────────────┐
│  4. Apply Phase 1    │
│                      │
│  Filter by repo      │
│  name and labels     │
└──────────────────────┘
            │
            ▼
┌──────────────────────┐
│  5. Fetch Branches   │
│                      │
│  Get branch info for │
│  filtered repos only │
└──────────────────────┘
            │
            ▼
┌──────────────────────┐
│  6. Apply Phase 2    │
│                      │
│  Filter by branch    │
│  name and paths      │
└──────────────────────┘
            │
            ▼
┌──────────────────────┐
│  7. Return Results   │
└──────────────────────┘
```

### Detailed Phase 1: Repository Filtering

```
Input: All repositories from SCM provider
       Filters categorized as "repo filters"

┌─────────────────────────────────────────┐
│  For each repository:                    │
│                                          │
│  ┌────────────────────────────────┐     │
│  │ For each repo filter:          │     │
│  │                                │     │
│  │ ┌──────────────────────────┐  │     │
│  │ │ Check repositoryMatch    │  │     │
│  │ │ Does repo name match?    │──┼──┐  │
│  │ └──────────────────────────┘  │  │  │
│  │                                │  │  │
│  │ ┌──────────────────────────┐  │  │  │
│  │ │ Check labelMatch         │  │  │  │
│  │ │ Do labels match?         │──┼──┤  │
│  │ └──────────────────────────┘  │  │  │
│  │                                │  │  │
│  │ All conditions match? ─────────┘  │  │
│  └────────────────────────────────┘  │  │
│                                       │  │
│  At least one filter matched? ────────┘  │
└─────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
    Include Repo            Exclude Repo

Output: Filtered repository list
```

### Detailed Phase 2: Branch Filtering

```
Input: Filtered repositories from Phase 1
       Filters categorized as "branch filters"

┌─────────────────────────────────────────┐
│  For each repository:                    │
│                                          │
│  ┌────────────────────────────────┐     │
│  │ For each branch filter:        │     │
│  │                                │     │
│  │ ┌──────────────────────────┐  │     │
│  │ │ Check branchMatch        │  │     │
│  │ │ Does branch name match?  │──┼──┐  │
│  │ └──────────────────────────┘  │  │  │
│  │                                │  │  │
│  │ ┌──────────────────────────┐  │  │  │
│  │ │ Check pathsExist         │  │  │  │
│  │ │ Do paths exist?          │  │  │  │
│  │ │ (API call for each path) │──┼──┤  │
│  │ └──────────────────────────┘  │  │  │
│  │                                │  │  │
│  │ ┌──────────────────────────┐  │  │  │
│  │ │ Check pathsDoNotExist    │  │  │  │
│  │ │ Are paths absent?        │  │  │  │
│  │ │ (API call for each path) │──┼──┤  │
│  │ └──────────────────────────┘  │  │  │
│  │                                │  │  │
│  │ All conditions match? ─────────┘  │  │
│  └────────────────────────────────┘  │  │
│                                       │  │
│  At least one filter matched? ────────┘  │
└─────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
    Include Repo            Exclude Repo

Output: Final filtered list
```

## Filter Logic: AND/OR Semantics

### Within a Single Filter (AND Logic)

All conditions within one filter must be satisfied:

```
┌─────────────────────────────────────┐
│         Single Filter               │
│                                     │
│  Condition A: repositoryMatch       │
│       AND                           │
│  Condition B: pathsExist            │
│       AND                           │
│  Condition C: branchMatch           │
│                                     │
│  Result: ALL must be true           │
└─────────────────────────────────────┘
```

**Example:**
```
Filter requires:
├─ Repository starts with "frontend-"
├─ Contains "package.json" file
└─ Branch is "main"

All three conditions must pass
```

### Between Multiple Filters (OR Logic)

At least one filter must match:

```
┌─────────────────────────────────────┐
│         Filter 1                    │
│  repositoryMatch: "^frontend-"      │
│  pathsExist: ["package.json"]       │
└─────────────────────────────────────┘
                OR
┌─────────────────────────────────────┐
│         Filter 2                    │
│  repositoryMatch: "^backend-"       │
│  pathsExist: ["go.mod"]             │
└─────────────────────────────────────┘

Result: Match if EITHER filter passes
```

**Logical Expression:**
```
(Filter 1 conditions) OR (Filter 2 conditions) OR (Filter 3 conditions)
```

## Example Walkthrough

### Configuration

```
Filter Definition:
├─ repositoryMatch: "^eks-"
└─ pathsExist: ["config/app.yaml"]
```

### Execution Flow

**Initial State:**
```
Available Repositories:
├─ eks-app-one
├─ eks-app-two
├─ other-app
└─ random-repo
```

**Phase 1: Repository Filtering**
```
Apply: repositoryMatch "^eks-"

eks-app-one    → Name starts with "eks-"     ✓ PASS
eks-app-two    → Name starts with "eks-"     ✓ PASS
other-app      → Name doesn't start with "eks-" ✗ FAIL
random-repo    → Name doesn't start with "eks-" ✗ FAIL

Phase 1 Result:
├─ eks-app-one  ✓
└─ eks-app-two  ✓
```

**Phase 2: Branch Filtering**
```
Apply: pathsExist ["config/app.yaml"]

For eks-app-one:
  Check: Does "config/app.yaml" exist?
  API Call Result: YES ✓
  Decision: PASS ✓

For eks-app-two:
  Check: Does "config/app.yaml" exist?
  API Call Result: NO ✗
  Decision: FAIL ✗

Phase 2 Result:
└─ eks-app-one  ✓
```

**Final Output:**
```
Matched Repositories:
└─ eks-app-one
```

## Performance Optimization Strategies

### Strategy 1: Filter Early

```
❌ Less Efficient:
┌──────────────────────────────┐
│ Filter only on pathsExist    │
│ Checks ALL repositories      │
│ Many expensive API calls     │
└──────────────────────────────┘

✓ More Efficient:
┌──────────────────────────────┐
│ Filter on repositoryMatch    │
│ THEN check pathsExist        │
│ Fewer API calls needed       │
└──────────────────────────────┘
```

### Strategy 2: Use Specific Patterns

```
❌ Overly Broad:
repositoryMatch: ".*"
└─ Matches everything, no filtering benefit

✓ Specific:
repositoryMatch: "^my-team-"
└─ Reduces repositories by 90%
```

### API Call Impact

```
Scenario: 1000 repositories, filter with pathsExist

Without Phase 1 filtering:
├─ API calls needed: 1000
└─ Time: ~500 seconds (rate limited)

With Phase 1 filtering (90% reduction):
├─ Repositories after Phase 1: 100
├─ API calls needed: 100
└─ Time: ~50 seconds (rate limited)

Performance Improvement: 10x faster
```

## Common Filter Patterns

### Pattern 1: Application Repositories

```
Use Case: Find all application repos with Kustomize on main branch

Filter Structure:
├─ repositoryMatch: "^app-"
├─ pathsExist: ["kustomization.yaml"]
└─ branchMatch: "^main$"

Evaluation Flow:
Phase 1: Filter by "^app-" → Reduces search space
Phase 2: Check for kustomization.yaml and main branch
```

### Pattern 2: Multiple Team Repositories

```
Use Case: Repositories from team-a OR team-b with ArgoCD label

Filter 1:
├─ repositoryMatch: "^team-a-"
└─ labelMatch: "^argocd$"

Filter 2:
├─ repositoryMatch: "^team-b-"
└─ labelMatch: "^argocd$"

Logic: (Filter 1) OR (Filter 2)
```

### Pattern 3: Exclude Archived

```
Use Case: All repositories that aren't archived

Filter Structure:
├─ repositoryMatch: ".*"
└─ pathsDoNotExist: [".archived", "DEPRECATED"]

Behavior: Includes any repo without archive markers
```

### Pattern 4: Specific Project Structure

```
Use Case: Helm charts that aren't opted out

Filter Structure:
├─ pathsExist: ["helm/Chart.yaml"]
└─ pathsDoNotExist: [".skip-argocd"]

Evaluation:
Phase 1: No filtering (no repo-level conditions)
Phase 2: Check for helm chart AND absence of skip file
```

## Filter Evaluation Summary

### Decision Tree

```
                    ┌─────────────────┐
                    │  Process Filter  │
                    └─────────────────┘
                            │
                ┌───────────┴───────────┐
                ▼                       ▼
        ┌──────────────┐        ┌──────────────┐
        │ Has Repo-    │        │ Has Branch-  │
        │ Level        │        │ Level        │
        │ Conditions?  │        │ Conditions?  │
        └──────────────┘        └──────────────┘
                │                       │
         ┌──────┴──────┐         ┌──────┴──────┐
         ▼             ▼         ▼             ▼
       YES            NO        YES            NO
         │             │         │             │
         ▼             └─────┐   ▼             │
    ┌─────────┐            │ ┌─────────┐      │
    │ Add to  │            │ │ Add to  │      │
    │ Repo    │            │ │ Branch  │      │
    │ Group   │            │ │ Group   │      │
    └─────────┘            │ └─────────┘      │
                           │                  │
                           └──────┬───────────┘
                                  ▼
                        ┌──────────────────┐
                        │ Filter is ready  │
                        │ for evaluation   │
                        └──────────────────┘
```

### Evaluation Order

```
1. Compilation
   └─ Convert YAML patterns to regex

2. Categorization
   ├─ Inspect conditions
   └─ Assign to phase groups

3. Phase 1 Execution
   ├─ Apply repo filters
   └─ Reduce repository set

4. Phase 2 Execution
   ├─ Fetch branches for filtered repos
   ├─ Apply branch filters
   └─ Perform path checks

5. Return Results
   └─ Final filtered repository list
```

## References

- Implementation: `applicationset/services/scm_provider/utils.go`
- Tests: `applicationset/services/scm_provider/utils_test.go`
