import {parseNumberFieldValue} from './number-field';

describe('parseNumberFieldValue', () => {
    test('returns undefined when the field is cleared', () => {
        expect(parseNumberFieldValue('')).toBeUndefined();
    });

    test('preserves zero so validation can reject it', () => {
        expect(parseNumberFieldValue('0')).toBe(0);
    });

    test('parses a numeric value', () => {
        expect(parseNumberFieldValue('123')).toBe(123);
    });
});
