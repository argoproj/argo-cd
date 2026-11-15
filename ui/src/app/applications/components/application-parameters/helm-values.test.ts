import {helmValuesError} from './helm-values';

describe('helmValuesError', () => {
    test('accepts a mapping', () => {
        expect(helmValuesError('components:\n  frontend:\n    image:\n      tag: abc123')).toBeNull();
    });

    test('accepts an empty document', () => {
        expect(helmValuesError(null)).toBeNull();
        expect(helmValuesError(undefined)).toBeNull();
        expect(helmValuesError('')).toBeNull();
        // Whitespace-only input used to throw as well: js-yaml rejects an empty document.
        expect(helmValuesError('   \n  ')).toBeNull();
    });

    test('rejects a scalar', () => {
        expect(helmValuesError('just-a-string')).toBe('Values must be a map');
    });

    test('reports duplicated mapping keys instead of throwing', () => {
        // Pasting the same block twice used to throw a YAMLException out of `validateError`,
        // which unmounted the React tree and blanked the UI (#30073).
        const duplicated = 'components:\n  frontend:\n    image:\n      tag: abc123\ncomponents:\n  frontend:\n    image:\n      tag: abc123';
        expect(() => helmValuesError(duplicated)).not.toThrow();
        expect(helmValuesError(duplicated)).toBe('Values are not valid YAML: duplicated mapping key');
    });

    test('reports other malformed YAML instead of throwing', () => {
        expect(helmValuesError('key: [unclosed')).toMatch(/^Values are not valid YAML: /);
    });
});
