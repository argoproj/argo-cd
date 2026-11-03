import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {formatTimestamp, Timestamp} from './timestamp';

const date = '2026-09-03T12:00:00Z';
// The date is formatted in local time, so build the expected value the same way the code does.
const expected = (locale?: string) =>
    new Date(date).toLocaleString(locale, {year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', numberingSystem: 'latn'});

test('formatTimestamp follows the locale', () => {
    expect(formatTimestamp(date, 'en-US')).toBe(expected('en-US'));
    expect(formatTimestamp(date, 'en-GB')).toBe(expected('en-GB'));
    // en-US puts the month first and en-GB the day first.
    expect(formatTimestamp(date, 'en-US')).not.toBe(formatTimestamp(date, 'en-GB'));
});

test('formatTimestamp keeps 0-9 digits for locales with another default numbering system', () => {
    expect(formatTimestamp(date, 'mr-IN')).toMatch(/^[0-9/., :APMapm -]+$/);
});

test('Timestamp shows the absolute date in the browser locale', () => {
    const html = renderToStaticMarkup(<Timestamp date={date} />);
    expect(html).toContain(expected());
    expect(html).not.toContain('GMT');
});
