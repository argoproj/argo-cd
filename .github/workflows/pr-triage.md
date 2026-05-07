---
on:
  ### Schedule would require an organization Copilot/AI license
  # schedule:
  #   - cron: '0 8,20 * * *' # 8am and 8pm UTC (twice daily)
  workflow_dispatch: # Manual trigger only
    inputs:
      analysis_refresh_threshold:
        description: 'Skip full analysis if cache is recent (e.g., "1h", "30m")'
        required: false
        default: '1h'
        type: string
      skip_checkpoint:
        description: 'Ignore checkpoints and start fresh'
        required: false
        default: false
        type: boolean

permissions:
  contents: read
  issues: read
  pull-requests: read

engine: copilot # AI engine to use

tools:
  github:
    toolsets: [default] # GitHub API access
    # github-app:
    #   app-id: ${{ secrets.PR_TRIAGE_GH_APP_CLIENT_ID }}
    #   private-key: ${{ secrets.PR_TRIAGE_GH_APP_PRIVATE_KEY }}
  cache-memory: true # Enable caching for faster subsequent runs

network: defaults

# See: https://github.github.com/gh-aw/reference/tokens/
safe-outputs:
  # github-app:
  #   app-id: ${{ secrets.PR_TRIAGE_GH_APP_CLIENT_ID }}
  #   private-key: ${{ secrets.PR_TRIAGE_GH_APP_PRIVATE_KEY }}
  update-project:
    project: https://github.com/orgs/argoproj/projects/38
    max: 50 # Update up to 50 PRs in the project
    target-repo: 'argoproj/argo-cd'
    views:
      - name: 'By Priority Score'
        layout: table
      - name: 'By Priority Tier'
        layout: board
      - name: 'By Category'
        layout: board
      - name: 'Critical & High Priority'
        layout: table
  create-issue: # Optional: for logging/reporting
    max: 1
---

# PR Priority Triage

Analyze all open pull requests in the argoproj/argo-cd repository, score them by priority, select the top 50, intelligently categorize them, and update the GitHub Project board.

## Objective

Help maintainers focus on the most important pull requests by:

1. Scoring all open PRs based on priority criteria
2. Selecting the top 50 PRs
3. Creating 5-10 adaptive categories based on what's in the top 50
4. Updating the project board with scores, tiers, and categories

## Priority Evaluation Criteria

### Critical Priority (Score 70+)

- PRs that are **approved and ready to merge** (highest priority)
- **Security fixes** or PRs linked to GHSA advisories
- Cherry-pick PRs **targeting release branches** (`release-*` pattern)
- PRs **associated with release milestones** matching VERSION file or previous semantic versions (planned work)

### High Priority (Score 50-69)

- PRs with **all CI checks passing**
- PRs from maintainers, **contributors** and `argoproj` org members
- **Bug fixes** with small code changes (<100 lines)

### Medium Priority (Score 30-49)

- PRs with open linked issues
- PRs waiting for initial review

### Low Priority (Score <30)

- PRs without linked issues (may be unsolicited)
- PRs from non-org members that appear AI-generated (check for generic descriptions, issue created shortly before PR)
- Large PRs (1000+ lines) requiring significant review time

### Deprioritize

- **Draft PRs** (not ready for review) - always exclude
- PRs with **merge conflicts** (needs author action first)
- **Stale PRs**: PRs that have received maintainer reviews/comments but author hasn't responded
  - 14 days since last author activity: slightly deprioritize (-5 points)
  - 30 days: medium deprioritization (-15 points)
  - 60+ days: significant deprioritization (-30 points)
  - **Important**: Only consider stale if there are reviews/comments from maintainers without subsequent author activity

## Guidelines

### Priority Scoring

- Be **conservative with "Critical"** - only truly urgent PRs
- **Consider context**: a large PR from a core maintainer may be higher priority than raw metrics suggest
- For **security PRs**, scan body text for GHSA references or security keywords (CVE, vulnerability, security advisory, security label)
- For **staleness**, only penalize if there are review comments without author response (don't penalize PRs waiting for initial review)

### Categorization (After selecting top 50 PRs)

1. **Review all 50 PRs** to understand themes and groupings
2. **Create 5-10 category names** that reflect what's actually in the list
3. **Make categories useful**: Group related work so specific maintainers/teams can focus on their areas
4. **Assign each PR** to exactly one category
5. **Good categories** typically have 3-15 PRs each

**Good category examples**:

- "Critical Security Fixes" (when multiple security PRs are top priority)
- "ApplicationSet Enhancements" (when several appset PRs are prioritized)
- "Sync Engine Improvements" (when sync-related work dominates)
- "Release Branch Fixes" (when preparing for release)
- "UI/UX Improvements" (when multiple UI PRs are prioritized)
- "CLI Enhancements" (when CLI work is prominent)

**Category guidance based on code areas**:

- Security: GHSA references, `security` label, CVE mentions
- Sync/Reconciliation: `controller/sync.go`, `controller/state.go`, sync-related issues
- ApplicationSet: `applicationset/` directory, appset-related issues
- CLI: `cmd/argocd/` directory
- UI: `ui/` directory, React/TypeScript changes
- Documentation: `docs/` directory, `*.md` files
- Hydrator: `hydrator/` directory
- Project/RBAC: project management, multi-tenancy, RBAC

**Avoid**:

- Creating too many categories (>10)
- Creating categories with only 1-2 PRs
- Using overly generic names like "Miscellaneous" for large groups (be specific about what the group represents)

### Output Actions

**DO**:

- ✅ Fetch all open PRs using GitHub API (exclude draft PRs and dependencies update)
- ✅ Score each PR based on the criteria above
- ✅ Select the top 50 PRs by score
- ✅ Analyze the top 50 to create meaningful categories
- ✅ Update the GitHub Project board with all 50 PRs
- ✅ Set custom fields for each PR:
  - **Priority Score**: numeric value (0-100+)
  - **Priority Tier**: "Critical", "High", or "Medium"
  - **Category**: your dynamically chosen category name
  - **Days Open**: calculated from creation date to today
  - **Key Factors**: concise text with emoji indicators (e.g., "🔴 Security, ✅ Approved, ✅ CI Pass")

**DO NOT**:

- ❌ Add labels to PRs
- ❌ Comment on PRs
- ❌ Modify PR titles or descriptions
- ❌ Create report issues or discussions
- ❌ Take any action on PRs themselves - ONLY update the project board

## Data Source

**IMPORTANT**: All PR data must be fetched from `argoproj/argo-cd`, NOT from the repository where this workflow is running.

When querying for PR information:

- Repository owner: `argoproj`
- Repository name: `argo-cd`
- Do NOT use the workflow's execution context (`${{ github.repository }}`)
  Use the GitHub API with explicit repository specification:
- `gh pr list --repo argoproj/argo-cd`
- GraphQL queries must use `owner: "argoproj"` and `repo: "argo-cd"`

## Checkpoint Resume Strategy

**At workflow start**:

1. Check if `skip_checkpoint` input is `true`:
   - If yes, ignore all checkpoint files and start fresh (do not delete them)
   - If no, proceed to checkpoint detection

2. **Resume from Pass 1 checkpoint**:
   - Check if `checkpoint-pass1.json` exists in cache
   - If found, load it and skip Pass 1 (PR list fetch, maintainer loading)
   - Use cached `prsAfterExclusions` data
   - Log: "Resumed from Pass 1 checkpoint: X PRs to analyze"
   - If not found, execute Pass 1 normally

3. **Resume from Pass 2 checkpoints**:
   - Check for `checkpoint-metadata.json` in cache
   - If not found, start Pass 2 from scratch
   - If found, check `pass2BatchesComplete` array in metadata
   - Load all completed batch files: `checkpoint-pass2-batch-N.json`
   - Merge all `prDetails` objects from completed batches
   - Calculate remaining batches: `pass2TotalBatches - pass2BatchesComplete.length`
   - Skip already-processed PR batches
   - Log: "Resumed from Pass 2: Y/X batches complete, Z remaining"

## Workflow Steps

### Caching Strategy

**Cache file location**: `/tmp/gh-aw/cache-memory/pr-scores.json`

**Quick Refresh Mode** (skip full analysis if cache is recent):

1. **Check workflow input** `analysis_refresh_threshold` (default: "1h")
2. **Load cache scores** from `/tmp/gh-aw/cache-memory/pr-scores.json`
3. **Parse time threshold** from input (e.g., "1h" → 1 hour, "30m" → 30 minutes, "2h" → 2 hours)
4. **Check cache age**: Compare current time with `metadata.lastAnalyzedAt`
5. **If cache is recent** (within threshold):
   - **Perform pass 1: Fetch current open PRs** (basic data only, apply exclusions)
   - **Skip Pass 2 and Pass 3** entirely
   - **Remove closed PRs** from the loaded cached scores - remove the PR if it is not in the Pass 1 result
   - **Perform pass 4** to select the new open top 50 PRs and update the board
   - Log: "Quick refresh mode: Reusing analysis from [timestamp], skipped detailed API calls"
6. **If cache is stale** (older than threshold) or doesn't exist:
   - Run full analysis

**Cache structure**:

```json
{
  "metadata": {
    "lastAnalyzedAt": "2026-04-22T08:00:00Z",
    "totalPRsAnalyzed": 680,
    "cacheVersion": "1.0"
  },
  "prs": {
    "26518": {
      "title": "fix: resolve CVE-2024-1234 in authentication flow",
      "score": 75,
      "tier": "High",
      "keyFactors": "🔴 Security, ✅ Approved, ✅ CI Pass",
      "changedFiles": ["server/auth/handler.go", "util/session/session.go"],
      "summary": "Fixes authentication vulnerability by adding proper input validation",
      "labels": ["security", "bug"]
    },
    "27059": {
      "title": "feat: add dark mode support to application details page",
      "score": 65,
      "tier": "Medium",
      "keyFactors": "🎨 UI, ⏳ Awaiting Review",
      "changedFiles": [
        "ui/src/app/applications/components/application-details.tsx",
        "ui/src/styles/theme.scss"
      ],
      "summary": "Implements dark mode theme support for application details view",
      "labels": ["enhancement", "ui"]
    }
  }
}
```

### Pass 1: Fetch Basic Data & Apply Exclusions

**Step 1: Load maintainer list**:

- Read `MAINTAINERS.md` file from repository
- Parse the markdown table to extract all GitHub usernames (column 2)
- Example maintainers: crenshaw-dev, alexmt, agaudreault, leoluz, etc.

**Step 2: Fetch all open PRs from argoproj/argo-cd**
Use the `list_pull_requests` MCP tool to collect all open PRs:

- Parameters: owner="argoproj", repo="argo-cd", state="open", perPage=100
- Fetch all pages sequentially (page 1, 2, 3, 4, 5, ...)
- Keep paging incrementally until a page is empty
- The MCP tool automatically handles JSON parsing - use the returned data directly

**Step 3: Filter out PRs to exclude**
From all the fetched PRs, remove:

1. Any PR with draft status
2. Any PR with label "dependencies"

**Save Pass 1 checkpoint**:

- Write to `/tmp/gh-aw/cache-memory/checkpoint-pass1.json`
- Include: `prsAfterExclusions` (array from list_pull_requests without the excluded PRs), `maintainers`
- Write to `/tmp/gh-aw/cache-memory/checkpoint-metadata.json`:
  - Set `createdAt` to current timestamp
  - Initialize `pass2BatchesComplete: []` and `pass2TotalBatches: <calculated>`
- Log: "Pass 1 checkpoint saved: X PRs to analyze"

### Pass 2: Fetch Detailed Data in Parallel Batches

**For all remaining PRs, fetch detailed data using MCP `pull_request_read` tool**:

**Batching strategy** (process 50 PRs at a time):

- Divide the filtered PRs into batches of 50 PRs each
- For each batch:

1. Check if batch is in checkpoint:
   - Check `checkpoint-metadata.json` for this batch number in `pass2BatchesComplete`
   - If found, load from `checkpoint-pass2-batch-N.json` instead of making API calls
   - Log: "Batch N loaded from checkpoint"

2. If not in checkpoint, fetch data for all PRs in batch in parallel:
   - For each PR, perform these calls sequentially:
     1. `pull_request_read` with `method: "get"` - PR details (mergeable_state, mergeable, rebaseable, head commit info)
     2. `pull_request_read` with `method: "get_reviews"` - Review and approval status
     3. `pull_request_read` with `method: "get_review_comments"` - Review comment threads with isResolved status
     4. `pull_request_read` with `method: "get_check_runs"` - CI/CD status checks
     5. `pull_request_read` with `method: "get_files"` - Changed files list and diff stats
   - Collect all detailed data

3. If not in checkpoint, **Save batch checkpoint**:
   - Write to `/tmp/gh-aw/cache-memory/checkpoint-pass2-batch-N.json`
   - Include: `batchNumber`, `prNumbers` (PRs in this batch), `prDetails` (object keyed by PR number)
   - Update `/tmp/gh-aw/cache-memory/checkpoint-metadata.json`:
     - Append `N` to `pass2BatchesComplete` array
   - Log: "Batch N checkpoint saved (Y/Z batches complete)"

### Pass 3: Score All PRs with Complete Data

Score all remaining PRs using the complete data collected in Pass 1 and Pass 2:

For each PR, collect metadata for categorization: title, changed files, labels, and a brief summary of the description.

For each PR, calculate priority score, tier and key factors based on:

- Approved reviews (highest boost)
- CI passing/failing status
- Merge conflicts/mergeable status
- Milestone matching and release branch targeting
- Security labels, bug severity
- Change size and scope
- Age and staleness
- Changes requested reviews (deprioritize)
- No recent updates (deprioritize if stale >1 month)
- **Author status** (contributor hierarchy scoring using `author_association` field):
  - Maintainer (highest boost): PR author username matches MAINTAINERS.md
  - Org member (high boost): `author_association` = `OWNER`, `COLLABORATOR`, or `MEMBER`
  - Contributor (medium boost): `author_association` = `CONTRIBUTOR`
  - First-time contributor (lower priority, careful review): `author_association` = `FIRST_TIME_CONTRIBUTOR`, `FIRST_TIMER`, `MANNEQUIN`, or `NONE`
- **Potential AI-generated PRs** (deprioritize if external contributor):
  - Check PR body description for AI-generated patterns:
    - **Vague or Robotic Descriptions**: Generic phrases like "This PR fixes the issue", "This PR addresses the problem", lack of specific technical details
    - **Overuse of Formatting**: Excessive markdown formatting, bullet points, headers, emojis that seem templated rather than natural
    - **Excessively Detailed**: Unnaturally verbose explanations, overly formal language, exhaustive lists that seem auto-generated
  - Apply AI-generation penalty ONLY if:
    - PR `author_association` is `FIRST_TIME_CONTRIBUTOR`, `FIRST_TIMER`, `MANNEQUIN`, or `NONE` (external contributors only)
    - AND PR body matches 2+ of the AI patterns above
  - Maintainers, org members, and contributors: no AI-generation penalty (trust established contributors)
- **Stale PRs awaiting author response**: Detect PRs with unresolved review feedback
  - Use the fetched result of `get_review_comments` and `get` of this PR
  - Check for unresolved review comment threads (`isResolved: false`)
  - Get last code push timestamp `pushedAt`
  - **PR is stale if**:
    - Has unresolved review comment threads (isResolved: false)
    - AND last code push was BEFORE the most recent unresolved comment
    - This indicates author hasn't addressed the feedback
  - Apply staleness penalty based on days since most recent unresolved comment:
    - 14+ days: -5 points (slightly stale)
    - 30+ days: -15 points (moderately stale)
    - 60+ days: -30 points (significantly stale)
  - **No staleness penalty if**:
    - No review comments yet (waiting for initial review)
    - All review threads are resolved (isResolved: true)
    - Code was pushed after most recent unresolved comment (author is actively responding)

**Save updated cache**:

- Write to `/tmp/gh-aw/cache-memory/pr-scores.json`
- Update `metadata.lastAnalyzedAt` to current timestamp
- Update `metadata.totalPRsAnalyzed` with count
- Store analyzed PRs in `prs` object (PR number as key) with these fields:
  - `title`: PR title
  - `score`: Priority score (0-100+)
  - `tier`: Priority tier (Critical/High/Medium/Low)
  - `keyFactors`: Emoji summary string (e.g., "🔴 Security, ✅ Approved, ✅ CI Pass")
  - `changedFiles`: Array of file paths that were modified in the PR
  - `summary`: Brief summary of the PR description to help with categorization
  - `labels`: Array of label names applied to the PR

### Pass 4: Select top 50, Categorize and Update Project Board

1. **Select top 50 PRs** by final score

2. **Analyze top 50 PRs** to identify natural groupings
   - Use the information from the cache score file (`pr-scores.json`):
     - PR `title` to understand what each PR does
     - `changedFiles` to see which code areas are affected
     - `summary` for context on the PR's purpose
     - `labels` to identify common themes
   - Identify patterns and themes across the top 50 PRs

3. **Create 5-10 category names** that reflect what's in the list

4. **Assign each top 50 PR to a category**

5. **Update project board** with all 50 PRs, setting custom fields (including the assigned category)

**Clean up checkpoints on success**:

- Delete all checkpoint files from cache:
  - `checkpoint-metadata.json`
  - `checkpoint-pass1.json`
  - All `checkpoint-pass2-batch-*.json` files
- Keep `pr-scores.json` (final scores cache)
- Log: "Checkpoints cleared after successful completion"

## Progress Reporting

Report progress at these checkpoints:

1. After fetching all pages: "Fetched X PRs across Y pages"
2. After filtering: "X PRs remaining after filtering dependency bots and drafts"
3. After fetching batches: "Completed batch X of Y"
4. After detailed scoring: "Completed detailed scoring, top 50 selected"
5. After categorization: "Created X categories for top 50 PRs"
6. After project update: "Updated project board with 50 PRs"

## Additional Context

- **Repository**: argoproj/argo-cd
- **Total open PRs**: ~800 open PRs, ~100 drafts, ~20 dependencies, ~165 with changes requested, ~425 without review, ~350 stale (>1 month), ~30-40 maintainers (in MAINTAINERS.md)
- **Argo CD areas**: Core controller, ApplicationSet controller, UI, CLI, sync engine, hydrator, RBAC, documentation
- **Contribution guidelines**: See CONTRIBUTING.md and AGENTS.md in the repository
- **Maintainers list**: see MAINTAINERS.md
- **Checkpoint retention**: Checkpoints are automatically cleared on successful completion
- **Resume capability**: Workflow can resume from last checkpoint if it fails mid-execution (saves ~5-10 minutes on retry)

## Performance Estimates

**Full Analysis Mode** (cache stale or doesn't exist):

- **Setup**: Read MAINTAINERS.md from repo (no API calls)
- **Pass 1**: ~8-10 API calls (pagination of list_pull_requests for ~800 PRs)
- **Pass 2**: ~680 filtered PRs × 5 calls = ~3400 API calls
- **Total**: ~3408-3410 API calls
- **Runtime**: 10-12 minutes with parallel batching

**Quick Refresh Mode** (cache age < analysis_refresh_threshold, default 1h):

Re-running within 1 hour to refresh board after PRs are closed/merged

- **Setup**: Read MAINTAINERS.md from repo (no API calls)
- **Pass 1**: ~8-10 API calls (pagination of list_pull_requests for ~800 PRs)
- **Pass 2 & 3**: SKIPPED (reuse cached scores)
- **Total**: ~8-10 API calls
- **Runtime**: <1 minute

**Resume from Checkpoint** (workflow failed mid-execution):

- **From Pass 1 checkpoint**: Skip ~8-10 API calls, save ~10-20 seconds
- **From Pass 2 checkpoint**: Skip already-completed batches
  - Example: 10/14 batches complete = skip ~2500 API calls, save ~7-8 minutes
- **Runtime**: Depends on how far workflow progressed before failure

## Success Criteria

- Top 50 PRs are scored accurately based on priority factors
- Categories are meaningful and help maintainers focus
- Each category has 3-15 PRs (balanced distribution)
- Security PRs and approved PRs are correctly prioritized
- Stale PRs are appropriately deprioritized
- Project board is updated with all 50 PRs and accurate custom field values
