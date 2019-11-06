import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {ResourceIcon} from './resource-icon';

test('ConfigMap', () => {
    const tree = renderer.create(<ResourceIcon kind='ConfigMap' />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('Application', () => {
    const tree = renderer.create(<ResourceIcon kind='Application' />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OneWord', () => {
    const tree = renderer.create(<ResourceIcon kind='OneWord' />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('TwoWords', () => {
    const tree = renderer.create(<ResourceIcon kind='TwoWords' />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ThreeWordsTotal', () => {
    const tree = renderer.create(<ResourceIcon kind='ThreeWordsTotal' />).toJSON();

    expect(tree).toMatchSnapshot();
});
