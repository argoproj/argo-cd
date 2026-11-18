import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {ProgressPopup} from './progress-popup';

function renderPopup(percentage: number, title: string) {
    return renderToStaticMarkup(<ProgressPopup onClose={() => {}} percentage={percentage} title={title} />);
}

test('ProgressPopup.0%', () => {
    const state = renderPopup(0, '');

    expect(state).toMatchSnapshot();
});

test('ProgressPopup.50%', () => {
    const state = renderPopup(50, 'My Title');

    expect(state).toMatchSnapshot();
});

test('ProgressPopup.100%', () => {
    const state = renderPopup(100, '');

    expect(state).toMatchSnapshot();
});
