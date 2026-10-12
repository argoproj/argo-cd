'use strict';

// Enforces the "every PR needs an issue" contribution policy. It is run by
// .github/workflows/pr-issue-policy.yaml through actions/github-script, and the
// policy itself is documented in docs/developer-guide/submit-your-pr.md.
//
// A pull request passes the policy when either:
//   - it changes fewer than MAX_EXEMPT_CODE_LINES lines outside of documentation
//     and generated files (this also covers documentation-only PRs), or
//   - it links at least one issue, and every linked issue labeled `enhancement`
//     also carries the ACCEPTED_LABEL label.
// A PR that fails the policy is labeled, gets an explanatory comment and is
// converted to a draft. Once it passes again, the PR is marked ready for review
// if, and only if, this script was the one that converted it to a draft.

const fs = require('fs');

const MAX_EXEMPT_CODE_LINES = 50;
const ENHANCEMENT_LABEL = 'enhancement';
const ACCEPTED_LABEL = 'triage/accepted';
const EXEMPT_LABEL = 'policy/exempt';
const NO_ISSUE_LABEL = 'policy/no-linked-issue';
const NOT_ACCEPTED_LABEL = 'policy/issue-not-accepted';
const POLICY_LABELS = [NO_ISSUE_LABEL, NOT_ACCEPTED_LABEL];
// PRs authored by users with write access to the repository are not checked.
const MAINTAINER_ASSOCIATIONS = ['OWNER', 'MEMBER', 'COLLABORATOR'];
const DOCS_PATTERNS = ['docs/**', '*.md', 'mkdocs.yml'];
// Upper bound on the number of referenced issues that are looked up.
const MAX_LINKED_ISSUES = 10;

const COMMENT_MARKER = '<!-- argocd-pr-issue-policy -->';
const CONVERTED_MARKER = '<!-- argocd-pr-issue-policy:converted-to-draft -->';
const POLICY_DOCS_URL =
  'https://argo-cd.readthedocs.io/en/latest/developer-guide/submit-your-pr/#linking-your-pr-to-an-issue';

const KEYWORDS = String.raw`(?:close[sd]?|fix(?:e[sd])?(?:\s+for)?|resolve[sd]?|address(?:e[sd])?|part\s+of|refs?|references|relate[sd]\s+to)`;
const REFERENCE = String.raw`(?:https?:\/\/github\.com\/([\w.-]+)\/([\w.-]+)\/issues\/(\d+)|([\w.-]+)\/([\w.-]+)#(\d+)|#(\d+))\b`;
const SEPARATOR = String.raw`\s*(?:,|&|\band\b)\s*`;

// Converts a gitignore-style pattern into a regular expression matching a
// repository-relative path. Patterns without a slash match at any depth.
function globToRegExp(glob) {
  let pattern = glob;
  if (pattern.startsWith('/')) {
    pattern = pattern.slice(1);
  } else if (!pattern.includes('/')) {
    pattern = `**/${pattern}`;
  }
  let re = '';
  for (let i = 0; i < pattern.length; i++) {
    const c = pattern[i];
    if (c === '*' && pattern[i + 1] === '*') {
      if (pattern[i + 2] === '/') {
        re += '(?:.*/)?';
        i += 2;
      } else {
        re += '.*';
        i += 1;
      }
    } else if (c === '*') {
      re += '[^/]*';
    } else if (c === '?') {
      re += '[^/]';
    } else {
      re += c.replace(/[.+^${}()|[\]\\]/g, '\\$&');
    }
  }
  return new RegExp(`^${re}$`);
}

// Returns the path patterns that .gitattributes marks as linguist-generated.
function parseGeneratedPatterns(gitattributes) {
  const patterns = [];
  for (const line of gitattributes.split('\n')) {
    const [pattern, ...attrs] = line.trim().split(/\s+/);
    if (!pattern || pattern.startsWith('#')) {
      continue;
    }
    if (attrs.includes('linguist-generated') || attrs.includes('linguist-generated=true')) {
      patterns.push(pattern);
    }
  }
  return patterns;
}

// Counts the changed lines (additions + deletions) that are neither
// documentation nor generated.
function countCodeLines(files, generatedPatterns) {
  const ignored = [...DOCS_PATTERNS, ...generatedPatterns].map(globToRegExp);
  return files
    .filter(f => !ignored.some(re => re.test(f.filename)))
    .reduce((sum, f) => sum + f.additions + f.deletions, 0);
}

// Removes HTML comments and code so that template placeholders and examples
// are not mistaken for issue references.
function stripNonProse(text) {
  return (text || '')
    .replace(/<!--[\s\S]*?(?:-->|$)/g, '')
    .replace(/```[\s\S]*?(?:```|$)/g, '')
    .replace(/`[^`\n]*`/g, '');
}

function collectReferences(text, owner, repo, numbers) {
  const re = new RegExp(REFERENCE, 'gi');
  for (const m of text.matchAll(re)) {
    const refOwner = m[1] || m[4];
    const refRepo = m[2] || m[5];
    if (refOwner && (refOwner.toLowerCase() !== owner.toLowerCase() || refRepo.toLowerCase() !== repo.toLowerCase())) {
      continue;
    }
    numbers.add(Number(m[3] || m[6] || m[7]));
  }
}

// Returns the numbers of the issues or PRs in owner/repo that the title or body
// refers to. In the body, only references following a linking keyword such as
// "Closes", "Fixes" or "Part of" count; in the title, every reference counts,
// because the title conventionally ends with the issue number, e.g. "(#123)".
function findReferencedNumbers(title, body, owner, repo) {
  const numbers = new Set();
  collectReferences(stripNonProse(title), owner, repo, numbers);
  const linked = new RegExp(String.raw`\b${KEYWORDS}:?\s+(${REFERENCE}(?:${SEPARATOR}${REFERENCE})*)`, 'gi');
  for (const m of stripNonProse(body).matchAll(linked)) {
    collectReferences(m[1], owner, repo, numbers);
  }
  return [...numbers];
}

// Decides whether a PR passes the policy. `issues` are the linked issues as
// returned by the REST API (pull requests already filtered out).
function evaluate({ codeLines, issues }) {
  if (codeLines < MAX_EXEMPT_CODE_LINES) {
    return { pass: true, reason: `changes ${codeLines} line(s) outside of documentation and generated files` };
  }
  if (issues.length === 0) {
    return { pass: false, label: NO_ISSUE_LABEL };
  }
  const hasLabel = (issue, name) => issue.labels.some(l => (typeof l === 'string' ? l : l.name) === name);
  const notAccepted = issues.filter(i => hasLabel(i, ENHANCEMENT_LABEL) && !hasLabel(i, ACCEPTED_LABEL));
  if (notAccepted.length > 0) {
    return { pass: false, label: NOT_ACCEPTED_LABEL, notAccepted: notAccepted.map(i => i.number) };
  }
  return { pass: true, reason: `links ${issues.map(i => `#${i.number}`).join(', ')}` };
}

function failureComment(result, { converted, draft, owner, repo }) {
  let state = 'This pull request should be a draft';
  if (converted) {
    state = 'This pull request has been converted to a draft';
  } else if (draft) {
    state = 'This pull request is a draft and should stay one';
  }
  const issueUrl = `https://github.com/${owner}/${repo}/issues/new/choose`;
  let why;
  let steps;
  if (result.label === NO_ISSUE_LABEL) {
    why = `${state} because it does not link to an issue. Maintainers do not routinely review drafts.

Pull requests that change ${MAX_EXEMPT_CODE_LINES} or more lines outside of documentation and generated files must link to an issue, so that maintainers can agree on the problem and the approach before effort goes into a review.`;
    steps = `1. Find the issue this PR addresses, or [open a new one](${issueUrl}). For an enhancement, wait until a maintainer adds the \`${ACCEPTED_LABEL}\` label.
2. Edit this PR's description to link the issue, for example \`Closes #1234\` or \`Part of #1234\`.
3. Mark the PR ready for review.`;
  } else {
    const issues = result.notAccepted.map(n => `#${n}`).join(', ');
    why = `${state} because the linked enhancement ${issues} has not been accepted yet: it is missing the \`${ACCEPTED_LABEL}\` label. Maintainers do not routinely review drafts.

Enhancements are triaged before implementation, so that a proposal is agreed on before effort goes into a review.`;
    steps = `1. Wait for a maintainer to triage ${issues} and add the \`${ACCEPTED_LABEL}\` label. You can raise it in the issue or at the [contributor meeting](https://argo-cd.readthedocs.io/en/latest/developer-guide/code-contributions/#regular-contributor-meeting).
2. Make sure this PR is consistent with the outcome of the discussion in the issue.
3. Mark the PR ready for review.`;
  }
  return `${COMMENT_MARKER}${converted ? `\n${CONVERTED_MARKER}` : ''}
### Automated policy notice

${why}

**To move forward:**

${steps}

The policy is checked again whenever the PR is edited or marked ready for review. See [Linking your PR to an issue](${POLICY_DOCS_URL}) for details.`;
}

function passComment(result, readied) {
  return `${COMMENT_MARKER}
### Automated policy notice

This pull request now satisfies the [issue policy](${POLICY_DOCS_URL}): it ${result.reason}.${readied ? ' It has been marked ready for review.' : ''}`;
}

async function listLinkedIssues({ github, owner, repo, pr }) {
  const numbers = new Set(findReferencedNumbers(pr.title, pr.body, owner, repo));
  // Also include issues linked through the "Development" sidebar.
  const { repository } = await github.graphql(
    `query($owner: String!, $repo: String!, $number: Int!) {
      repository(owner: $owner, name: $repo) {
        pullRequest(number: $number) {
          closingIssuesReferences(first: ${MAX_LINKED_ISSUES}) {
            nodes { number repository { nameWithOwner } }
          }
        }
      }
    }`,
    { owner, repo, number: pr.number },
  );
  for (const node of repository.pullRequest.closingIssuesReferences.nodes) {
    if (node.repository.nameWithOwner.toLowerCase() === `${owner}/${repo}`.toLowerCase()) {
      numbers.add(node.number);
    }
  }

  const issues = [];
  for (const number of [...numbers].slice(0, MAX_LINKED_ISSUES)) {
    if (number === pr.number) {
      continue;
    }
    try {
      const { data } = await github.rest.issues.get({ owner, repo, issue_number: number });
      if (!data.pull_request) {
        issues.push(data);
      }
    } catch (e) {
      if (e.status !== 404 && e.status !== 410) {
        throw e;
      }
    }
  }
  return issues;
}

async function findPolicyComment({ github, owner, repo, pr }) {
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner,
    repo,
    issue_number: pr.number,
    per_page: 100,
  });
  return comments.find(c => c.user && c.user.type === 'Bot' && c.body && c.body.startsWith(COMMENT_MARKER));
}

async function upsertComment({ github, owner, repo, pr, existing, body }) {
  if (existing) {
    if (existing.body !== body) {
      await github.rest.issues.updateComment({ owner, repo, comment_id: existing.id, body });
    }
  } else {
    await github.rest.issues.createComment({ owner, repo, issue_number: pr.number, body });
  }
}

async function setPolicyLabel({ github, core, owner, repo, pr, label }) {
  const current = pr.labels.map(l => l.name);
  for (const stale of POLICY_LABELS.filter(l => l !== label && current.includes(l))) {
    await github.rest.issues.removeLabel({ owner, repo, issue_number: pr.number, name: stale });
  }
  if (label && !current.includes(label)) {
    try {
      await github.rest.issues.addLabels({ owner, repo, issue_number: pr.number, labels: [label] });
    } catch (e) {
      // A missing label must not prevent the PR from being converted to a draft.
      core.warning(`Could not add label ${label}: ${e.message}`);
    }
  }
}

async function run({ github, context, core, gitattributes }) {
  const pr = context.payload.pull_request;
  const { owner, repo } = context.repo;

  if (pr.user.type === 'Bot' || MAINTAINER_ASSOCIATIONS.includes(pr.author_association)) {
    core.info(`Skipping: #${pr.number} is authored by ${pr.user.login} (${pr.author_association}).`);
    return;
  }

  let result;
  if (pr.labels.some(l => l.name === EXEMPT_LABEL)) {
    result = { pass: true, reason: `carries the \`${EXEMPT_LABEL}\` label` };
  } else {
    const files = await github.paginate(github.rest.pulls.listFiles, {
      owner,
      repo,
      pull_number: pr.number,
      per_page: 100,
    });
    const attributes = gitattributes ?? (fs.existsSync('.gitattributes') ? fs.readFileSync('.gitattributes', 'utf8') : '');
    const codeLines = countCodeLines(files, parseGeneratedPatterns(attributes));
    const issues = codeLines < MAX_EXEMPT_CODE_LINES ? [] : await listLinkedIssues({ github, owner, repo, pr });
    result = evaluate({ codeLines, issues });
  }
  core.info(`Policy result for #${pr.number}: ${JSON.stringify(result)}`);

  const existing = await findPolicyComment({ github, owner, repo, pr });
  const convertedEarlier = Boolean(existing && existing.body.includes(CONVERTED_MARKER));

  if (!result.pass) {
    await setPolicyLabel({ github, core, owner, repo, pr, label: result.label });
    let convertError;
    if (!pr.draft) {
      try {
        await github.graphql(
          `mutation($id: ID!) { convertPullRequestToDraft(input: { pullRequestId: $id }) { clientMutationId } }`,
          { id: pr.node_id },
        );
      } catch (e) {
        convertError = e;
      }
    }
    const converted = (!pr.draft && !convertError) || convertedEarlier;
    const body = failureComment(result, { converted, draft: pr.draft, owner, repo });
    await upsertComment({ github, owner, repo, pr, existing, body });
    if (convertError) {
      // The author still gets the comment, but the failure must be visible to maintainers.
      core.setFailed(`Could not convert #${pr.number} to a draft: ${convertError.message}`);
    }
    return;
  }

  await setPolicyLabel({ github, core, owner, repo, pr, label: null });
  if (!existing) {
    return;
  }
  // Only undo a conversion this script made; a draft the author opened stays a draft.
  const readied = pr.draft && convertedEarlier;
  if (readied) {
    await github.graphql(
      `mutation($id: ID!) { markPullRequestReadyForReview(input: { pullRequestId: $id }) { clientMutationId } }`,
      { id: pr.node_id },
    );
  }
  await upsertComment({ github, owner, repo, pr, existing, body: passComment(result, readied) });
}

module.exports = run;
Object.assign(module.exports, {
  ACCEPTED_LABEL,
  COMMENT_MARKER,
  CONVERTED_MARKER,
  NO_ISSUE_LABEL,
  NOT_ACCEPTED_LABEL,
  countCodeLines,
  evaluate,
  findReferencedNumbers,
  globToRegExp,
  parseGeneratedPatterns,
});
