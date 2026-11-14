import {namespaceQuery, namespaceQueryKey} from './applications-service.namespace';

describe('ApplicationSet namespace query params', () => {
    it('uses appNamespace for applications', () => {
        expect(namespaceQueryKey('application')).toBe('appNamespace');
        expect(namespaceQuery('application', 'team-platform')).toEqual({appNamespace: 'team-platform'});
    });

    it('uses appsetNamespace for ApplicationSet GET/resource-tree', () => {
        expect(namespaceQueryKey('applicationset')).toBe('appsetNamespace');
        expect(namespaceQuery('applicationset', 'team-platform')).toEqual({appsetNamespace: 'team-platform'});
    });

    it('uses appSetNamespace for ApplicationSet watch stream', () => {
        expect(namespaceQueryKey('applicationset', true)).toBe('appSetNamespace');
        expect(namespaceQuery('applicationset', 'team-platform', true)).toEqual({appSetNamespace: 'team-platform'});
    });

    it('returns empty object when namespace is empty', () => {
        expect(namespaceQuery('applicationset', '')).toEqual({});
    });
});
