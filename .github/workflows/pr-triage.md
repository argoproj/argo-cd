---
on:
  schedule:
    - cron: '0 8,20 * * *' # 8am and 8pm UTC (twice daily)
  workflow_dispatch: # Manual trigger

permissions:
  contents: read
  issues: read
  pull-requests: read

engine: copilot # AI engine to use

tools:
  github:
    toolsets: [default] # GitHub API access
    github-app:
      app-id: ${{ secrets.PR_TRIAGE_GH_APP_ID }}
      private-key: ${{ secrets.PR_TRIAGE_GH_APP_PRIVATE_KEY }}

network: defaults

safe-outputs:
  github-app:
    app-id: ${{ secrets.PR_TRIAGE_GH_APP_ID }}
    private-key: ${{ secrets.PR_TRIAGE_GH_APP_PRIVATE_KEY }}
  update-project:
    project: https://github.com/users/agaudreault/projects/1
    max: 50 # Update up to 50 PRs in the project
    target-repo: 'agaudreault/argo-cd'
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

- PRs that are **approved and ready to merge** (highest priority - blocks release)
- **Security fixes** or PRs linked to GHSA advisories
- PRs **targeting release branches** (easier to merge, unblocks releases)
- PRs **associated with release milestones** matching VERSION file or previous semantic versions

### High Priority (Score 50-69)

- PRs with **all CI checks passing** and linked to approved issues
- PRs from **frequent contributors** (check merged PR history using `gh pr list --author USERNAME --state merged --limit 100`)
- **Bug fixes** with small code changes (<100 lines)

### Medium Priority (Score 30-49)

- PRs with linked issues that follow contribution guidelines
- PRs waiting for initial review (not stale)

### Low Priority (Score <30)

- PRs without linked issues (may be unsolicited)
- PRs from non-org members that appear AI-generated (check for generic descriptions, issue created shortly before PR)
- Large PRs (1000+ lines) requiring significant review time

### Deprioritize

- **Draft PRs** (not ready for review) - exclude from top 50
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
- For **security PRs**, scan body text for GHSA references or security keywords (CVE, vulnerability, security advisory)
- Check **org membership** with `gh api orgs/argoproj/members/{username}` (404 = not a member)
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

- ✅ Fetch all open PRs using GitHub API (exclude draft PRs)
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

## Workflow Steps

1. **Fetch all open PRs** from argoproj/argo-cd using `gh pr list --repo argoproj/argo-cd --state open --draft=false --limit 1000 --json ...`

2. **For each PR, gather data**:
   - Title, number, author, creation date, update date
   - Draft status, mergeable status
   - Linked issues (from PR body)
   - Labels
   - Milestone
   - Reviews and review comments
   - Status checks (CI results)
   - Additions/deletions (change size)
   - Files (Changed files)
   - Base ref (target branch)

3. **Calculate priority score** for each PR based on the criteria

4. **Sort by score** and select top 50

5. **Analyze top 50 PRs** to identify natural groupings

6. **Create 5-10 category names** that reflect what's in the list

7. **Assign each PR to a category**

8. **Update project board** with all 50 PRs, setting custom fields

## Additional Context

- **Repository**: argoproj/argo-cd
- **Total open PRs**: Usually 200-300
- **Argo CD areas**: Core controller, ApplicationSet controller, UI, CLI, sync engine, hydrator, RBAC, documentation
- **Contribution guidelines**: See CONTRIBUTING.md and AGENTS.md in the repository
- **Common maintainers**: see MAINTAINERS.md

## Success Criteria

- Top 50 PRs are scored accurately based on priority factors
- Categories are meaningful and help maintainers focus
- Each category has 3-15 PRs (balanced distribution)
- Security PRs and approved PRs are correctly prioritized
- Stale PRs are appropriately deprioritized
- Project board is updated with all 50 PRs and accurate custom field values
