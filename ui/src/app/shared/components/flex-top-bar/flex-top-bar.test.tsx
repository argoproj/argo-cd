import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {getTooltipContent, renderActionMenuLabel} from './flex-top-bar';

function renderLabel(title: string | React.ReactElement) {
    return renderToStaticMarkup(<React.Fragment>{renderActionMenuLabel(title)}</React.Fragment>);
}

test('getTooltipContent.string', () => {
    expect(getTooltipContent('Sync')).toBe('Sync');
});

test('getTooltipContent.emptyString', () => {
    expect(getTooltipContent('')).toBe('');
});

test('getTooltipContent.element', () => {
    expect(getTooltipContent(<span>Custom</span>)).toBeUndefined();
});

test('renderActionMenuLabel.string', () => {
    const label = renderLabel('Sync');

    expect(label).toMatchSnapshot();
});

test('renderActionMenuLabel.element', () => {
    const label = renderLabel(<span>Custom</span>);

    expect(label).toMatchSnapshot();
});
