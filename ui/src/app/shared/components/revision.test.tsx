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
