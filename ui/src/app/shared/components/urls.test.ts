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

test('bitbucket.org', () => {
    testExample(
        'https://alexcollinsinuit@bitbucket.org/alexcollinsinuit/test-repo.git',
        'git@bitbucket.org:alexcollinsinuit/test-repo.git',
        '38fb93957deb45ff546af13399a92ac0d568c350',
        'https://bitbucket.org/alexcollinsinuit/test-repo',
        'https://bitbucket.org/alexcollinsinuit/test-repo/commits/38fb93957deb45ff546af13399a92ac0d568c350',
    );
});

test('empty url', () => {
    expect(repoUrl('')).toBe(null);
});
