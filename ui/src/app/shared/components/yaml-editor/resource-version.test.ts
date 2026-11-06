import {hasReachedResourceVersion, resourceVersionOf} from './resource-version';

describe('resourceVersionOf', () => {
    test('reads the resource version from a Kubernetes object', () => {
        expect(resourceVersionOf({metadata: {resourceVersion: '42'}})).toBe('42');
    });

    test('returns null when there is nothing usable to compare against', () => {
        expect(resourceVersionOf(undefined)).toBeNull();
        expect(resourceVersionOf(null)).toBeNull();
        expect(resourceVersionOf(true)).toBeNull();
        expect(resourceVersionOf({})).toBeNull();
        expect(resourceVersionOf({metadata: {}})).toBeNull();
        expect(resourceVersionOf({metadata: {resourceVersion: ''}})).toBeNull();
        expect(resourceVersionOf({spec: {replicas: 3}})).toBeNull();
    });
});

describe('hasReachedResourceVersion', () => {
    test('is reached at the exact version and at any later one', () => {
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '250'}}, '250')).toBe(true);
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '251'}}, '250')).toBe(true);
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '1000'}}, '250')).toBe(true);
    });

    test('is not reached while the live resource is older', () => {
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '249'}}, '250')).toBe(false);
    });

    test('orders by magnitude rather than by plain string order', () => {
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '1000'}}, '999')).toBe(true);
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '999'}}, '1000')).toBe(false);
    });

    test('stays exact for versions too large to be held by a JS number', () => {
        // Both of these collapse to the same float, so parsing them as numbers would call the older
        // live resource caught up.
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '9007199254740992'}}, '9007199254740993')).toBe(false);
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '9007199254740993'}}, '9007199254740992')).toBe(true);
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '2345678901234567890123456789012345678901'}}, '345678901234567890123456789012345678901')).toBe(true);
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '345678901234567890123456789012345678900'}}, '345678901234567890123456789012345678901')).toBe(false);
    });

    test('falls back to equality when a version is not an orderable integer', () => {
        // Extension API servers may serve versions that cannot be ordered at all.
        expect(hasReachedResourceVersion({metadata: {resourceVersion: 'opaque'}}, 'opaque')).toBe(true);
        expect(hasReachedResourceVersion({metadata: {resourceVersion: 'opaque'}}, '250')).toBe(false);
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '250'}}, 'opaque')).toBe(false);
        expect(hasReachedResourceVersion({metadata: {resourceVersion: '0250'}}, '250')).toBe(false);
        expect(hasReachedResourceVersion({}, '250')).toBe(false);
    });
});
