'use strict';

// Run with: node --test .github/scripts/

const assert = require('node:assert/strict');
const { describe, it } = require('node:test');

const run = require('./pr-issue-policy.js');
const {
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
} = run;

describe('globToRegExp', () => {
  const cases = [
    ['docs/**', 'docs/index.md', true],
    ['docs/**', 'docs/assets/a/b.png', true],
    ['docs/**', 'mydocs/index.md', false],
    ['*.md', 'README.md', true],
    ['*.md', 'ui/src/README.md', true],
    ['*.md', 'README.md.go', false],
    ['**/*.pb.go', 'pkg/apiclient/session.pb.go', true],
    ['**/*.pb.go', 'session.pb.go', true],
    ['**/mocks/*.go', 'util/git/mocks/Client.go', true],
    ['**/mocks/*.go', 'util/git/mocks/sub/Client.go', false],
    ['manifests/crds/*-crd.yaml', 'manifests/crds/application-crd.yaml', true],
    ['manifests/crds/*-crd.yaml', 'manifests/crds/kustomization.yaml', false],
    ['/mkdocs.yml', 'mkdocs.yml', true],
    ['mkdocs.yml', 'docs/mkdocs.yml', true],
    ['assets/swagger.json', 'assets/swagger.json', true],
    ['assets/swagger.json', 'assets/swagger_json', false],
  ];
  for (const [glob, path, want] of cases) {
    it(`${glob} ${want ? 'matches' : 'does not match'} ${path}`, () => {
      assert.equal(globToRegExp(glob).test(path), want);
    });
  }
});

describe('parseGeneratedPatterns', () => {
  it('returns only the patterns marked as generated', () => {
    const attrs = [
      '# comment',
      '',
      '**/*.pb.go linguist-generated=true',
      'assets/swagger.json   linguist-generated',
      'hack/foo.sh -linguist-generated',
      'hack/bar.sh linguist-generated=false',
      '*.sh text eol=lf',
    ].join('\n');
    assert.deepEqual(parseGeneratedPatterns(attrs), ['**/*.pb.go', 'assets/swagger.json']);
  });
});

describe('countCodeLines', () => {
  it('ignores documentation and generated files', () => {
    const files = [
      { filename: 'docs/operator-manual/index.md', additions: 400, deletions: 10 },
      { filename: 'docs/assets/screenshot.png', additions: 0, deletions: 0 },
      { filename: 'README.md', additions: 5, deletions: 5 },
      { filename: 'mkdocs.yml', additions: 2, deletions: 0 },
      { filename: 'pkg/apis/application/v1alpha1/generated.pb.go', additions: 900, deletions: 100 },
      { filename: 'controller/state.go', additions: 30, deletions: 12 },
      { filename: 'ui/src/app/app.tsx', additions: 3, deletions: 1 },
    ];
    assert.equal(countCodeLines(files, ['**/*.pb.go']), 46);
  });
});

describe('findReferencedNumbers', () => {
  const find = (title, body) => findReferencedNumbers(title, body, 'argoproj', 'argo-cd').sort((a, b) => a - b);

  it('finds closing and reference keywords in the body', () => {
    const body = [
      'Closes #1',
      'fixes: #2',
      'Resolved #3',
      'Part of #4',
      'Refs #5',
      'Related to #6',
      'relates to argoproj/argo-cd#7',
      'Fixes https://github.com/argoproj/argo-cd/issues/8',
      'Possible fix for #9',
      'Addresses #10',
    ].join('\n');
    assert.deepEqual(find('fix: something', body), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
  });

  it('finds lists of references after a single keyword', () => {
    assert.deepEqual(find('fix: something', 'Fixes #10, #11 and #12 & #13'), [10, 11, 12, 13]);
  });

  it('ignores references without a keyword in the body', () => {
    assert.deepEqual(find('fix: something', 'This is similar to #20, see also #21.'), []);
  });

  it('finds references in the title', () => {
    assert.deepEqual(find('fix: resolve OutOfSync bug (#30)', ''), [30]);
  });

  it('ignores other repositories', () => {
    const body = 'Fixes argoproj/argo-rollouts#40\nCloses https://github.com/akuity/kargo/issues/41\nRefs ARGOPROJ/Argo-CD#42';
    assert.deepEqual(find('fix: x (argoproj/gitops-engine#43)', body), [42]);
  });

  it('ignores pull request URLs', () => {
    assert.deepEqual(find('fix: x', 'Related to https://github.com/argoproj/argo-cd/pull/50'), []);
  });

  it('ignores HTML comments and code', () => {
    const body = [
      '<!-- Closes #60 -->',
      'Closes #<!-- issue number -->',
      '`Fixes #61`',
      '```',
      'Fixes #62',
      '```',
    ].join('\n');
    assert.deepEqual(find('fix: x', body), []);
  });

  it('handles an empty body', () => {
    assert.deepEqual(find('fix: x', null), []);
  });
});

const issue = (number, ...labels) => ({ number, labels: labels.map(name => ({ name })) });

describe('evaluate', () => {
  it('passes small changes without an issue', () => {
    assert.equal(evaluate({ codeLines: 49, issues: [] }).pass, true);
  });

  it('fails changes of 50 lines or more without an issue', () => {
    assert.deepEqual(evaluate({ codeLines: 50, issues: [] }), { pass: false, label: NO_ISSUE_LABEL });
  });

  it('passes with a linked bug', () => {
    assert.equal(evaluate({ codeLines: 500, issues: [issue(1, 'bug')] }).pass, true);
  });

  it('passes with a linked accepted enhancement', () => {
    assert.equal(evaluate({ codeLines: 500, issues: [issue(1, 'enhancement', ACCEPTED_LABEL)] }).pass, true);
  });

  it('fails when any linked enhancement is not accepted', () => {
    const result = evaluate({
      codeLines: 500,
      issues: [issue(1, 'bug'), issue(2, 'enhancement', ACCEPTED_LABEL), issue(3, 'enhancement')],
    });
    assert.deepEqual(result, { pass: false, label: NOT_ACCEPTED_LABEL, notAccepted: [3] });
  });
});

// fakeGitHub records the calls run() makes against the GitHub API.
function fakeGitHub({ files, issues = {}, sidebar = [], comments = [], convertError }) {
  const calls = [];
  const record = name => async args => {
    calls.push({ name, args });
    return { data: {} };
  };
  const github = {
    calls,
    paginate: async (fn, args) => fn(args),
    graphql: async (query, vars) => {
      if (query.includes('closingIssuesReferences')) {
        return {
          repository: {
            pullRequest: {
              closingIssuesReferences: {
                nodes: sidebar.map(number => ({ number, repository: { nameWithOwner: 'argoproj/argo-cd' } })),
              },
            },
          },
        };
      }
      calls.push({ name: query.includes('convertPullRequestToDraft') ? 'convertToDraft' : 'markReady', args: vars });
      if (convertError && query.includes('convertPullRequestToDraft')) {
        throw convertError;
      }
      return {};
    },
    rest: {
      pulls: { listFiles: async () => files },
      issues: {
        get: async ({ issue_number }) => {
          if (!issues[issue_number]) {
            const err = new Error('Not Found');
            err.status = 404;
            throw err;
          }
          return { data: issues[issue_number] };
        },
        listComments: async () => comments,
        createComment: record('createComment'),
        updateComment: record('updateComment'),
        addLabels: record('addLabels'),
        removeLabel: record('removeLabel'),
      },
    },
  };
  return github;
}

const core = { info() {}, warning() {}, setFailed() {} };
const context = pr => ({
  repo: { owner: 'argoproj', repo: 'argo-cd' },
  payload: {
    pull_request: {
      number: 100,
      node_id: 'PR_node',
      title: 'feat: something',
      body: '',
      draft: false,
      labels: [],
      author_association: 'CONTRIBUTOR',
      user: { login: 'someone', type: 'User' },
      ...pr,
    },
  },
});
const bigChange = [{ filename: 'controller/state.go', additions: 80, deletions: 0 }];
const names = github => github.calls.map(c => c.name);

describe('run', () => {
  it('converts a PR without an issue to a draft', async () => {
    const github = fakeGitHub({ files: bigChange });
    await run({ github, context: context({}), core, gitattributes: '' });
    assert.deepEqual(names(github), ['addLabels', 'convertToDraft', 'createComment']);
    assert.deepEqual(github.calls[0].args.labels, [NO_ISSUE_LABEL]);
    assert.ok(github.calls[2].args.body.includes(CONVERTED_MARKER));
  });

  it('still comments and fails the job when the PR cannot be converted to a draft', async () => {
    const github = fakeGitHub({ files: bigChange, convertError: new Error('Resource not accessible by integration') });
    const failures = [];
    await run({ github, context: context({}), core: { ...core, setFailed: m => failures.push(m) }, gitattributes: '' });
    assert.deepEqual(names(github), ['addLabels', 'convertToDraft', 'createComment']);
    assert.ok(!github.calls[2].args.body.includes(CONVERTED_MARKER));
    assert.ok(github.calls[2].args.body.includes('This pull request should be a draft'));
    assert.equal(failures.length, 1);
  });

  it('does not convert a PR linking an accepted enhancement', async () => {
    const github = fakeGitHub({ files: bigChange, issues: { 7: issue(7, 'enhancement', ACCEPTED_LABEL) } });
    await run({ github, context: context({ body: 'Closes #7' }), core, gitattributes: '' });
    assert.deepEqual(names(github), []);
  });

  it('converts a PR linking an unaccepted enhancement from the sidebar', async () => {
    const github = fakeGitHub({ files: bigChange, issues: { 7: issue(7, 'enhancement') }, sidebar: [7] });
    await run({ github, context: context({}), core, gitattributes: '' });
    assert.deepEqual(names(github), ['addLabels', 'convertToDraft', 'createComment']);
    assert.deepEqual(github.calls[0].args.labels, [NOT_ACCEPTED_LABEL]);
  });

  it('does not count a referenced pull request as an issue', async () => {
    const github = fakeGitHub({ files: bigChange, issues: { 8: { ...issue(8, 'bug'), pull_request: {} } } });
    await run({ github, context: context({ body: 'Refs #8' }), core, gitattributes: '' });
    assert.deepEqual(names(github), ['addLabels', 'convertToDraft', 'createComment']);
  });

  it('does not count generated files towards the size limit', async () => {
    const files = [{ filename: 'pkg/apis/application/v1alpha1/generated.pb.go', additions: 900, deletions: 0 }];
    const github = fakeGitHub({ files });
    await run({ github, context: context({}), core, gitattributes: '**/*.pb.go linguist-generated=true' });
    assert.deepEqual(names(github), []);
  });

  it('leaves a draft opened by the author as a draft', async () => {
    const github = fakeGitHub({ files: bigChange });
    await run({ github, context: context({ draft: true }), core, gitattributes: '' });
    assert.deepEqual(names(github), ['addLabels', 'createComment']);
    assert.ok(!github.calls[1].args.body.includes(CONVERTED_MARKER));
  });

  it('marks a PR it converted ready for review once it passes', async () => {
    const existing = {
      id: 1,
      user: { type: 'Bot' },
      body: `${COMMENT_MARKER}\n${CONVERTED_MARKER}\nold`,
    };
    const github = fakeGitHub({ files: bigChange, issues: { 9: issue(9, 'bug') }, comments: [existing] });
    const pr = { draft: true, body: 'Fixes #9', labels: [{ name: NO_ISSUE_LABEL }] };
    await run({ github, context: context(pr), core, gitattributes: '' });
    assert.deepEqual(names(github), ['removeLabel', 'markReady', 'updateComment']);
  });

  it('does not mark a draft ready that it did not convert', async () => {
    const existing = { id: 1, user: { type: 'Bot' }, body: `${COMMENT_MARKER}\nold` };
    const github = fakeGitHub({ files: bigChange, issues: { 9: issue(9, 'bug') }, comments: [existing] });
    const pr = { draft: true, body: 'Fixes #9', labels: [{ name: NO_ISSUE_LABEL }] };
    await run({ github, context: context(pr), core, gitattributes: '' });
    assert.deepEqual(names(github), ['removeLabel', 'updateComment']);
  });

  it('honours the exempt label', async () => {
    const github = fakeGitHub({ files: bigChange });
    await run({ github, context: context({ labels: [{ name: 'policy/exempt' }] }), core, gitattributes: '' });
    assert.deepEqual(names(github), []);
  });

  it('skips maintainers and bots', async () => {
    for (const pr of [{ author_association: 'MEMBER' }, { user: { login: 'renovate[bot]', type: 'Bot' } }]) {
      const github = fakeGitHub({ files: bigChange });
      await run({ github, context: context(pr), core, gitattributes: '' });
      assert.deepEqual(names(github), []);
    }
  });
});
