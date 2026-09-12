/* eslint-env jest */
declare const test: any;
declare const expect: any;
declare const describe: any;
import {concatMaps} from './utils';
import {gotoIfQueryChanged, isValidManagedByURL, isValidURL} from './utils';

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

describe('gotoIfQueryChanged', () => {
    const navigateFrom = (search: string, params: {[name: string]: any}) => {
        window.history.replaceState({}, '', '/applications' + search);
        const goto = jest.fn();
        gotoIfQueryChanged({goto} as any, params);
        return goto;
    };

    test('replaces the URL when a value changes', () => {
        const goto = navigateFrom('?proj=default', {proj: 'other'});
        expect(goto).toHaveBeenCalledWith('.', {proj: 'other'}, {replace: true});
    });

    test('replaces the URL when a parameter is added', () => {
        expect(navigateFrom('?proj=default', {proj: 'default', showFavorites: 'true'})).toHaveBeenCalled();
    });

    test('does not navigate when every parameter already has that value', () => {
        expect(navigateFrom('?proj=default&health=Healthy', {proj: 'default', health: 'Healthy'})).not.toHaveBeenCalled();
    });

    test('does not navigate for parameters it was not given', () => {
        expect(navigateFrom('?proj=default&view=tiles', {proj: 'default'})).not.toHaveBeenCalled();
    });

    test('treats null and undefined as removing a parameter', () => {
        expect(navigateFrom('?showFavorites=true', {showFavorites: null})).toHaveBeenCalled();
        expect(navigateFrom('?proj=default', {showFavorites: null})).not.toHaveBeenCalled();
        expect(navigateFrom('?proj=default', {showFavorites: undefined})).not.toHaveBeenCalled();
    });

    test('compares every entry of an array parameter', () => {
        expect(navigateFrom('?type=git&type=helm', {type: ['git', 'helm']})).not.toHaveBeenCalled();
        expect(navigateFrom('?type=git&type=helm', {type: ['git']})).toHaveBeenCalled();
        expect(navigateFrom('?type=git', {type: ['git', 'helm']})).toHaveBeenCalled();
    });

    test('navigates when the URL has no query string yet', () => {
        expect(navigateFrom('', {proj: ''})).toHaveBeenCalled();
        expect(navigateFrom('', {proj: null})).not.toHaveBeenCalled();
    });
});
