import {parseHelmValues, validateHelmValues} from './helm-values';

describe('Helm values validation', () => {
    test('accepts a YAML map', () => {
        expect(validateHelmValues('image:\n  tag: stable')).toBeUndefined();
        expect(parseHelmValues('image:\n  tag: stable').value).toEqual({image: {tag: 'stable'}});
    });

    test('rejects malformed YAML', () => {
        expect(validateHelmValues('image:ef\n  tag: stable')).toBe('Values must be valid YAML');
    });

    test.each(['- item', 'null', '2026-09-04'])('rejects non-map YAML: %s', values => {
        expect(validateHelmValues(values)).toBe('Values must be a map');
    });
});