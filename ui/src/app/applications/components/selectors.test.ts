/**
 * Test cases for annotation and label selectors
 */
import {createMetadataSelector} from './selectors';

describe('Selectors', () => {
    describe('createMetadataSelector', () => {
        it('should match annotations with key=value format', () => {
            const annotations = {team: 'core'};
            const selector = createMetadataSelector(['team=core'], 'annotation');
            expect(selector(annotations)).toBe(true);
        });

        it('should match annotations with key only format', () => {
            const annotations = {team: 'core'};
            const selector = createMetadataSelector(['team'], 'annotation');
            expect(selector(annotations)).toBe(true);
        });

        it('should not match annotations with wrong value', () => {
            const annotations = {team: 'core'};
            const selector = createMetadataSelector(['team=ops'], 'annotation');
            expect(selector(annotations)).toBe(false);
        });

        it('should not match when annotation key does not exist', () => {
            const annotations = {};
            const selector = createMetadataSelector(['team'], 'annotation');
            expect(selector(annotations)).toBe(false);
        });

        it('should match version annotation correctly', () => {
            const annotations = {version: 'v2'};
            const selector = createMetadataSelector(['version=v2'], 'annotation');
            expect(selector(annotations)).toBe(true);
        });

        it('should support multiple selectors with AND logic', () => {
            const annotations = {team: 'core', version: 'v2'};
            const selector = createMetadataSelector(['team=core', 'version=v2'], 'annotation');
            expect(selector(annotations)).toBe(true);
        });

        it('should fail when one of multiple selectors does not match', () => {
            const annotations = {team: 'core', version: 'v1'};
            const selector = createMetadataSelector(['team=core', 'version=v2'], 'annotation');
            expect(selector(annotations)).toBe(false);
        });

        it('should work with labels as well', () => {
            const labels = {env: 'production', tier: 'frontend'};
            const selector = createMetadataSelector(['env=production', 'tier'], 'label');
            expect(selector(labels)).toBe(true);
        });
    });
});