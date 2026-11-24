import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {isSHA, Revision} from './revision';

function renderJson(element: React.ReactElement) {
    return renderToStaticMarkup(element);
}

test('Revision.SHA1.Children', () => {
    const tree = renderJson(
        <Revision repoUrl='http://github.com/my-org/my-repo' revision='24eb0b24099b2e9afff72558724e88125eaa0176'>
            foo
        </Revision>,
    );

    expect(tree).toMatchSnapshot();
});

test('Revision.SHA1.NoChildren', () => {
    const tree = renderJson(<Revision repoUrl='http://github.com/my-org/my-repo' revision='24eb0b24099b2e9afff72558724e88125eaa0176' />);

    expect(tree).toMatchSnapshot();
});

test('Revision.Branch.Children', () => {
    const tree = renderJson(
        <Revision repoUrl='http://github.com/my-org/my-repo' revision='long-branch-name'>
            foo
        </Revision>,
    );

    expect(tree).toMatchSnapshot();
});

test('Revision.Branch.NoChildren', () => {
    const tree = renderJson(<Revision repoUrl='http://github.com/my-org/my-repo' revision='long-branch-name' />);

    expect(tree).toMatchSnapshot();
});

test('isSHA1', () => {
    expect(isSHA('24eb0b24099b2e9afff72558724e88125eaa0176')).toBe(true);
    expect(isSHA('master')).toBe(false);
});

test('Revision.AzureDevOps.CommitLink', () => {
    const tree = renderJson(
        <Revision
            repoUrl='https://example-org@dev.azure.com/example-org/example-project/_git/example-repo'
            revision='36582dddea28a6ced2b0fdeedceb12f5945bde36'
        />,
    );

    expect(tree).toContain(
        'href="https://dev.azure.com/example-org/example-project/_git/example-repo/commit/36582dddea28a6ced2b0fdeedceb12f5945bde36"',
    );
    expect(tree).toContain('36582dd');
    expect(tree).toContain('<a ');
});

test('Revision.AzureDevOps.BranchFileLink', () => {
    const tree = renderJson(
        <Revision repoUrl='https://dev.azure.com/example/project/_git/repo' revision='main' path='carwash/overlays/auto' />,
    );

    expect(tree).toContain('version=GBmain&amp;path=%2Fcarwash%2Foverlays%2Fauto');
});

test('Revision.AzureDevOps.CommitFileLink', () => {
    const tree = renderJson(
        <Revision repoUrl='https://dev.azure.com/example/project/_git/repo' revision='36582dddea28a6ced2b0fdeedceb12f5945bde36' path='README.md' />,
    );

    expect(tree).toContain('version=GC36582dddea28a6ced2b0fdeedceb12f5945bde36&amp;path=%2FREADME.md');
});

test('Revision.AzureDevOps.FilePathEncoding', () => {
    const tree = renderJson(
        <Revision repoUrl='https://dev.azure.com/example/project/_git/repo' revision='main' path='config/my file#1&2.yaml' />,
    );

    expect(tree).toContain('version=GBmain&amp;path=%2Fconfig%2Fmy+file%231%262.yaml');
});
