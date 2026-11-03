import {repoUrl, revisionUrl} from './urls';

function testExample(http: string, ssl: string, revision: string, expectedRepoUrl: string, expectedRevisionUrl: string) {
    expect(repoUrl(http)).toBe(expectedRepoUrl);
    expect(repoUrl(ssl)).toBe(expectedRepoUrl);
    expect(revisionUrl(http, revision)).toBe(expectedRevisionUrl);
    expect(revisionUrl(ssl, revision)).toBe(expectedRevisionUrl);
    expect(repoUrl(http)).toBe(expectedRepoUrl);
    expect(revisionUrl(http, revision)).toBe(expectedRevisionUrl);
    expect(revisionUrl(ssl, revision)).toBe(expectedRevisionUrl);
}

test('github.com', () => {
    testExample(
        'https://github.com/argoproj/argo-cd.git',
        'git@github.com:argoproj/argo-cd.git',
        '024dee09f543ce7bb5af7ca50260504d89dfda94',
        'https://github.com/argoproj/argo-cd',
        'https://github.com/argoproj/argo-cd/commit/024dee09f543ce7bb5af7ca50260504d89dfda94',
    );
});

// for enterprise github installations
test('github.my-enterprise.com', () => {
    testExample(
        'https://github.my-enterprise.com/my-org/my-repo.git',
        'git@github.my-enterprise.com:my-org/my-repo.git',
        'a06f2be80a4da89abb8ced904beab75b3ec6db0e',
        'https://github.my-enterprise.com/my-org/my-repo',
        'https://github.my-enterprise.com/my-org/my-repo/commit/a06f2be80a4da89abb8ced904beab75b3ec6db0e',
    );
});

test('gitlab.com', () => {
    testExample(
        'https://gitlab.com/alex_collins/private-repo.git',
        'git@gitlab.com:alex_collins/private-repo.git',
        'b1fe9426ead684d7af16958920968342ee295c1f',
        'https://gitlab.com/alex_collins/private-repo',
        'https://gitlab.com/alex_collins/private-repo/-/commit/b1fe9426ead684d7af16958920968342ee295c1f',
    );
});

// for self-hosted gitlab installations
test('gitlab.my-enterprise.com', () => {
    testExample(
        'https://gitlab.my-enterprise.com/my-org/my-repo.git',
        'git@gitlab.my-enterprise.com:my-org/my-repo.git',
        'b1fe9426ead684d7af16958920968342ee295c1f',
        'https://gitlab.my-enterprise.com/my-org/my-repo',
        'https://gitlab.my-enterprise.com/my-org/my-repo/-/commit/b1fe9426ead684d7af16958920968342ee295c1f',
    );
});

// lookalike hostnames must not be treated as GitLab
test('gitlab lookalike host is not supported', () => {
    expect(repoUrl('https://gitlabmalicious.com/my-org/my-repo.git')).toBe(null);
    expect(repoUrl('git@gitlabmalicious.com:my-org/my-repo.git')).toBe(null);
    expect(revisionUrl('https://gitlabmalicious.com/my-org/my-repo.git', 'b1fe9426ead684d7af16958920968342ee295c1f', false)).toBe(null);
    expect(revisionUrl('git@gitlabmalicious.com:my-org/my-repo.git', 'b1fe9426ead684d7af16958920968342ee295c1f', false)).toBe(null);
});

test('bitbucket.org', () => {
    testExample(
        'https://alexcollinsinuit@bitbucket.org/alexcollinsinuit/test-repo.git',
        'git@bitbucket.org:alexcollinsinuit/test-repo.git',
        '38fb93957deb45ff546af13399a92ac0d568c350',
        'https://bitbucket.org/alexcollinsinuit/test-repo',
        'https://bitbucket.org/alexcollinsinuit/test-repo/commits/38fb93957deb45ff546af13399a92ac0d568c350',
    );
});

// for self-hosted Bitbucket Server installations
// Clone URLs use /scm/ prefix; browse URLs use /users/ or /projects/ prefix.
test('bitbucket server (self-hosted, personal repo, HTTPS)', () => {
    expect(repoUrl('https://bitbucket.example.com/scm/~user/repo-name.git')).toBe(
        'https://bitbucket.example.com/users/user/repos/repo-name',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/~user/repo-name.git', 'abc1234', false)).toBe(
        'https://bitbucket.example.com/users/user/repos/repo-name/commits/abc1234',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/~user/repo-name.git', 'main', false)).toBe(
        'https://bitbucket.example.com/users/user/repos/repo-name/browse?at=main',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/~user/repo-name.git', 'v1.0.0', false)).toBe(
        'https://bitbucket.example.com/users/user/repos/repo-name/browse?at=v1.0.0',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/~user/repo-name.git', '', false)).toBe(
        'https://bitbucket.example.com/users/user/repos/repo-name/browse',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/~user/repo-name.git', 'HEAD', false)).toBe(
        'https://bitbucket.example.com/users/user/repos/repo-name/browse',
    );
});

test('bitbucket server (self-hosted, project repo, HTTPS)', () => {
    expect(repoUrl('https://bitbucket.example.com/scm/MYPROJECT/repo-name.git')).toBe(
        'https://bitbucket.example.com/projects/MYPROJECT/repos/repo-name',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/MYPROJECT/repo-name.git', 'abc1234', false)).toBe(
        'https://bitbucket.example.com/projects/MYPROJECT/repos/repo-name/commits/abc1234',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/MYPROJECT/repo-name.git', 'main', false)).toBe(
        'https://bitbucket.example.com/projects/MYPROJECT/repos/repo-name/browse?at=main',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/MYPROJECT/repo-name.git', 'v1.0.0', false)).toBe(
        'https://bitbucket.example.com/projects/MYPROJECT/repos/repo-name/browse?at=v1.0.0',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/MYPROJECT/repo-name.git', '', false)).toBe(
        'https://bitbucket.example.com/projects/MYPROJECT/repos/repo-name/browse',
    );
    expect(revisionUrl('https://bitbucket.example.com/scm/MYPROJECT/repo-name.git', 'HEAD', false)).toBe(
        'https://bitbucket.example.com/projects/MYPROJECT/repos/repo-name/browse',
    );
});

test('bitbucket server (self-hosted, SSH)', () => {
    expect(repoUrl('git@bitbucket.example.com:scm/~user/repo-name.git')).toBe(
        'https://bitbucket.example.com/users/user/repos/repo-name',
    );
    expect(repoUrl('git@bitbucket.example.com:scm/MYPROJECT/repo-name.git')).toBe(
        'https://bitbucket.example.com/projects/MYPROJECT/repos/repo-name',
    );
});

test('bitbucket server (self-hosted, HTTP with explicit port)', () => {
    expect(repoUrl('http://bitbucket.example.com:7990/scm/MYPROJECT/repo-name.git')).toBe(
        'http://bitbucket.example.com:7990/projects/MYPROJECT/repos/repo-name',
    );
    expect(revisionUrl('http://bitbucket.example.com:7990/scm/MYPROJECT/repo-name.git', 'main', false)).toBe(
        'http://bitbucket.example.com:7990/projects/MYPROJECT/repos/repo-name/browse?at=main',
    );
});

test('empty url', () => {
    expect(repoUrl('')).toBe(null);
});

test('Azure DevOps HTTPS and SSH repository and commit links', () => {
    const urls = [
        'https://example-org@dev.azure.com/example-org/example-project/_git/example-repo',
        'https://dev.azure.com/example-org/example-project/_git/example-repo',
        'git@ssh.dev.azure.com:v3/example-org/example-project/example-repo'
    ];
    const expectedRepoUrl = 'https://dev.azure.com/example-org/example-project/_git/example-repo';
    const revision = '36582dddea28a6ced2b0fdeedceb12f5945bde36';

    for (const url of urls) {
        expect(repoUrl(url)).toBe(expectedRepoUrl);
        expect(revisionUrl(url, revision, false)).toBe(`${expectedRepoUrl}/commit/${revision}`);
    }
});

test('Azure DevOps lookalike domains are not supported', () => {
    const url = 'https://dev.azure.com.example.org/example-org/example-project/_git/example-repo';

    expect(repoUrl(url)).toBe(null);
    expect(revisionUrl(url, '36582dd', false)).toBe(null);
});

test('Azure DevOps legacy HTTPS and SSH repository and commit links', () => {
    const urls = ['https://example.visualstudio.com/project/_git/repo', 'git@vs-ssh.visualstudio.com:v3/example/project/repo'];
    const expectedRepoUrl = 'https://dev.azure.com/example/project/_git/repo';
    const revision = '36582dddea28a6ced2b0fdeedceb12f5945bde36';

    for (const url of urls) {
        expect(repoUrl(url)).toBe(expectedRepoUrl);
        expect(revisionUrl(url, revision, false)).toBe(`${expectedRepoUrl}/commit/${revision}`);
    }
});


test('Azure DevOps legacy lookalike domains are not supported', () => {
    const urls = ['https://example.visualstudio.com.evil.org/project/_git/repo', 'https://visualstudio.com.evil.org/project/_git/repo'];

    for (const url of urls) {
        expect(repoUrl(url)).toBe(null);
        expect(revisionUrl(url, '36582dd', false)).toBe(null);
    }
});

test('Azure DevOps branch, tag and file revisions', () => {
    const repo = 'https://dev.azure.com/example/project/_git/repo';
    const sha = '36582dddea28a6ced2b0fdeedceb12f5945bde36';

    expect(revisionUrl(repo, 'main', false)).toBe(`${repo}?version=GBmain`);
    expect(revisionUrl(repo, 'feature/test', false)).toBe(`${repo}?version=GBfeature%2Ftest`);
    expect(revisionUrl(repo, 'refs/heads/main', false)).toBe(`${repo}?version=GBmain`);
    expect(revisionUrl(repo, 'refs/tags/v1.0.0', false)).toBe(`${repo}?version=GTv1.0.0`);
    expect(revisionUrl(repo, sha, true)).toBe(`${repo}?version=GC${sha}`);
    expect(revisionUrl(repo, 'HEAD', false)).toBe(repo);
    expect(revisionUrl(repo, '', false)).toBe(repo);
});
