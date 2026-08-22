import {createMatcher} from './resources-list-search';

describe('resources createMatcher', () => {
    describe('empty search', () => {
        test('returns a matcher that always matches, regardless of regex flag', () => {
            expect(createMatcher('', false)('argocd-server', 'argocd')).toBe(true);
            expect(createMatcher('', true)('argocd-server', 'argocd')).toBe(true);
        });
    });

    describe('plain substring search', () => {
        test('matches substring in name', () => {
            expect(createMatcher('server', false)('argocd-server', 'argocd')).toBe(true);
        });

        test('matches substring in namespace', () => {
            expect(createMatcher('kube', false)('coredns', 'kube-system')).toBe(true);
        });

        test('returns false when neither name nor namespace contains the term', () => {
            expect(createMatcher('redis', false)('argocd-server', 'argocd')).toBe(false);
        });

        test('is case-sensitive', () => {
            expect(createMatcher('SERVER', false)('argocd-server', 'argocd')).toBe(false);
        });

        test('treats regex metacharacters as literals', () => {
            expect(createMatcher('^argocd', false)('argocd-server', 'argocd')).toBe(false);
        });

        test('tolerates a missing (cluster-scoped) namespace', () => {
            expect(createMatcher('cluster', false)('cluster-role', undefined)).toBe(true);
            expect(createMatcher('role', false)('cluster-role', undefined)).toBe(true);
        });
    });

    describe('regex search', () => {
        test('matches anchored pattern on name', () => {
            const matcher = createMatcher('^argocd-', true);
            expect(matcher('argocd-server', 'argocd')).toBe(true);
            expect(matcher('redis-argocd', 'argocd')).toBe(false);
        });

        test('matches anchored pattern on namespace', () => {
            expect(createMatcher('system$', true)('coredns', 'kube-system')).toBe(true);
        });

        test('matches alternation', () => {
            const matcher = createMatcher('^(argocd-server|redis)', true);
            expect(matcher('redis', 'argocd')).toBe(true);
            expect(matcher('argocd-repo-server', 'argocd')).toBe(false);
        });

        test('returns false for invalid regex', () => {
            expect(createMatcher('[abc', true)('argocd-server', 'argocd')).toBe(false);
            expect(createMatcher('*', true)('argocd-server', 'argocd')).toBe(false);
        });

        test('is case-sensitive (no implicit "i" flag)', () => {
            expect(createMatcher('^ARGOCD-', true)('argocd-server', 'argocd')).toBe(false);
        });

        test('tolerates a missing (cluster-scoped) namespace', () => {
            expect(createMatcher('^cluster', true)('cluster-role', undefined)).toBe(true);
        });
    });
});
