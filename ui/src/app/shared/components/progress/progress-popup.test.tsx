import * as renderer from 'react-test-renderer';
import * as React from 'react';
import {ProgressPopup} from './progress-popup';


test('ProgressPopup.0%', () => {
    const state = renderer.create(<ProgressPopup onClose={() => {
    }} percentage={0} title={''}/>);

    expect(state).toMatchSnapshot();
});
