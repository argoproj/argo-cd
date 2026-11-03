import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {englishLocaleFor, formatDate, formatTimestamp, Timestamp} from './timestamp';

const date = '2026-09-03T12:00:00Z';
// The date is formatted in local time, so build the expected value the same way the code does.
const expected = (locale: string) =>
    new Date(date).toLocaleString(locale, {year: 'numeric', month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', numberingSystem: 'latn'});

test('englishLocaleFor keeps the region and switches the language to English', () => {
    expect(englishLocaleFor('mr-IN')).toBe('en-IN');
    expect(englishLocaleFor('de-DE')).toBe('en-DE');
    expect(englishLocaleFor('en-GB')).toBe('en-GB');
    expect(englishLocaleFor('de')).toBe('en');
    expect(englishLocaleFor('not a locale')).toBe('en');
});

test('formatTimestamp follows the region', () => {
    expect(formatTimestamp(date, 'en-US')).toBe(expected('en-US'));
    expect(formatTimestamp(date, 'en-GB')).toBe(expected('en-GB'));
    expect(formatTimestamp(date, 'en-US')).not.toBe(formatTimestamp(date, 'en-GB'));
});

test('formatTimestamp spells out the month in English for any locale', () => {
    expect(formatTimestamp(date, 'en-US')).toContain('Sep');
    expect(formatTimestamp(date, 'en-US')).not.toContain('09/');
    expect(formatTimestamp(date, 'fr-FR')).toContain('Sep');
    // mr-IN is formatted as en-IN: English month and 0-9 digits instead of Marathi.
    expect(formatTimestamp(date, 'mr-IN')).toBe(expected('en-IN'));
    expect(formatTimestamp(date, 'mr-IN')).not.toMatch(/[ऀ-ॿ]/);
});

test('formatDate shows only the date with a textual month', () => {
    expect(formatDate(date, 'en-US')).toBe(new Date(date).toLocaleDateString('en-US', {year: 'numeric', month: 'short', day: '2-digit', numberingSystem: 'latn'}));
    expect(formatDate(date, 'en-US')).not.toContain(':');
});

test('Timestamp shows the absolute date in English for the browser region', () => {
    const html = renderToStaticMarkup(<Timestamp date={date} />);
    expect(html).toContain(expected(englishLocaleFor()));
    expect(html).not.toContain('GMT');
});
