/* eslint-env jest */
declare const test: any;
declare const expect: any;
declare const describe: any;
import {concatMaps} from './utils';
import {isValidManagedByURL, isValidURL} from './utils';

test('map concatenation', () => {
    const map1 = {
        a: '1',
        b: '2',
    };
    const map2 = {
        a: '9',
        c: '8',
    };
    const map3 = concatMaps(map1, map2);
    expect(map3).toEqual(new Map(Object.entries({a: '9', b: '2', c: '8'})));
});

describe('isValidURL', () => {
    test('accepts http/https URLs', () => {
        expect(isValidURL('http://example.com')).toBe(true);
        expect(isValidURL('https://example.com/path?q=1')).toBe(true);
    });

    test('accepts relative URLs', () => {
        // @ts-ignore
        window.location = new URL('https://localhost:8080/applications');
        expect(isValidURL('/applications')).toBe(true);
    });

    test('rejects unsafe protocols', () => {
        expect(isValidURL('javascript:alert(1)')).toBe(false);
        expect(isValidURL('JaVaScRiPt:alert(1)')).toBe(false);
        expect(isValidURL('data:text/html,<script>alert(1)</script>')).toBe(false);
        expect(isValidURL('vbscript:msgbox(1)')).toBe(false);
    });
});

describe('isValidManagedByURL', () => {
    test('accepts http/https URLs', () => {
        expect(isValidManagedByURL('http://example.com')).toBe(true);
        expect(isValidManagedByURL('https://example.com')).toBe(true);
        expect(isValidManagedByURL('https://localhost:8081')).toBe(true);
    });

    test('rejects non-http(s) protocols', () => {
        expect(isValidManagedByURL('ftp://localhost:8081')).toBe(false);
        expect(isValidManagedByURL('file:///etc/passwd')).toBe(false);
        expect(isValidManagedByURL('javascript:alert(1)')).toBe(false);
        expect(isValidManagedByURL('data:text/html,<script>alert(1)</script>')).toBe(false);
        expect(isValidManagedByURL('vbscript:msgbox(1)')).toBe(false);
    });

    test('rejects invalid URL strings', () => {
        expect(isValidManagedByURL('not-a-url')).toBe(false);
        expect(isValidManagedByURL('')).toBe(false);
    });
});
