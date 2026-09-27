import {createMatcher} from './applications-list-search';

describe('createMatcher', () => {
    describe('empty search', () => {
        test('returns a matcher that always matches, regardless of regex flag', () => {
            expect(createMatcher('', false)('app-prod-frontend', 'default')).toBe(true);
            expect(createMatcher('', true)('app-prod-frontend', 'default')).toBe(true);
        });
    });

    describe('plain substring search', () => {
        test('matches substring in name', () => {
            expect(createMatcher('prod', false)('app-prod-frontend', 'default')).toBe(true);
        });

        test('matches substring in namespace', () => {
            expect(createMatcher('platform', false)('guestbook', 'team-platform')).toBe(true);
        });

        test('returns false when neither name nor namespace contains the term', () => {
            expect(createMatcher('staging', false)('app-prod-frontend', 'default')).toBe(false);
        });

        test('is case-sensitive', () => {
            expect(createMatcher('PROD', false)('app-prod-frontend', 'default')).toBe(false);
        });

        test('treats regex metacharacters as literals', () => {
            expect(createMatcher('^app', false)('app-prod-frontend', 'default')).toBe(false);
        });
    });

    describe('regex search', () => {
        test('matches anchored pattern on name', () => {
            const matcher = createMatcher('^app-prod-', true);
            expect(matcher('app-prod-frontend', 'default')).toBe(true);
            expect(matcher('guestbook-prod', 'default')).toBe(false);
        });

        test('matches anchored pattern on namespace', () => {
            expect(createMatcher('frontend$', true)('guestbook', 'team-frontend')).toBe(true);
        });

        test('matches alternation', () => {
            const matcher = createMatcher('^(app-prod|guestbook)', true);
            expect(matcher('guestbook-prod', 'default')).toBe(true);
            expect(matcher('app-staging-frontend', 'default')).toBe(false);
        });

        test('returns false for invalid regex', () => {
            expect(createMatcher('[abc', true)('app-prod-frontend', 'default')).toBe(false);
            expect(createMatcher('*', true)('app-prod-frontend', 'default')).toBe(false);
        });

        test('is case-sensitive (no implicit "i" flag)', () => {
            expect(createMatcher('^APP-', true)('app-prod-frontend', 'default')).toBe(false);
        });

        test('compiles the regex once and reuses it across calls', () => {
            const matcher = createMatcher('^app-', true);
            expect(matcher('app-one', 'default')).toBe(true);
            expect(matcher('app-two', 'default')).toBe(true);
            expect(matcher('guestbook', 'default')).toBe(false);
        });
    });
});
