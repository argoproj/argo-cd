import {ResourceLabel} from './resource-label';

test('ConfigMap', () => {
    expect(ResourceLabel({kind: 'ConfigMap'})).toBe('cm');
});

test('Word', () => {
    expect(ResourceLabel({kind: 'Word'})).toBe('word');
});

test('LongWord', () => {
    expect(ResourceLabel({kind: 'Longword'})).toBe('longwor');
});

test('TwoWords', () => {
    expect(ResourceLabel({kind: 'TwoWords'})).toBe('t-words');
});

test('ThreeWordsTotal', () => {
    expect(ResourceLabel({kind: 'ThreeWordsTotal'})).toBe('twt');
});

