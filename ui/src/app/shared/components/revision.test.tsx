import * as renderer from 'react-test-renderer';
import * as React from 'react';
import {isSHA, Revision} from './revision';

test('Revision.SHA1.Children', () => {
    const tree = renderer
        .create(
            <Revision repoUrl='http://github.com/my-org/my-repo' revision='24eb0b24099b2e9afff72558724e88125eaa0176'>
                foo
            </Revision>,
        )
        .toJSON();

    expect(tree).toMatchSnapshot();
});

test('Revision.SHA1.NoChildren', () => {
    const tree = renderer.create(<Revision repoUrl='http://github.com/my-org/my-repo' revision='24eb0b24099b2e9afff72558724e88125eaa0176' />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('Revision.Branch.Children', () => {
    const tree = renderer
        .create(
            <Revision repoUrl='http://github.com/my-org/my-repo' revision='long-branch-name'>
                foo
            </Revision>,
        )
        .toJSON();

    expect(tree).toMatchSnapshot();
});

test('Revision.Branch.NoChildren', () => {
    const tree = renderer.create(<Revision repoUrl='http://github.com/my-org/my-repo' revision='long-branch-name' />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('isSHA1', () => {
    expect(isSHA('24eb0b24099b2e9afff72558724e88125eaa0176')).toBe(true);
    expect(isSHA('master')).toBe(false);
});
