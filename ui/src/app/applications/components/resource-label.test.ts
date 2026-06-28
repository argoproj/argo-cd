import {ResourceLabel} from './resource-label';

test('BuiltIn', () => {
    expect(ResourceLabel({kind: 'ConfigMap'})).toBe('cm');
});

test('CustomResource', () => {
    expect(ResourceLabel({kind: 'Word'})).toBe('word');
});
