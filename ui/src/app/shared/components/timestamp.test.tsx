import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {Timestamp} from './timestamp';

test('Timestamp shows the absolute date as year-month-day', () => {
    const html = renderToStaticMarkup(<Timestamp date='2026-09-03T12:00:00Z' />);
    expect(html).toMatch(/2026-09-03 \d{2}:\d{2}:\d{2}/);
    expect(html).not.toContain('GMT');
});
