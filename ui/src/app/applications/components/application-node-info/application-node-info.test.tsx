import {stripManagedFields} from './application-node-info';

describe('stripManagedFields', () => {
    it('removes metadata.managedFields', () => {
        const resource = {
            metadata: {
                name: 'my-deployment',
                managedFields: [{manager: 'argocd-controller', operation: 'Update'}]
            }
        };

        const result = stripManagedFields(resource);

        expect(result.metadata.managedFields).toBeUndefined();
    });

    it('removes the argocd.argoproj.io/tracking-id annotation', () => {
        const resource = {
            metadata: {
                name: 'my-deployment',
                annotations: {
                    'argocd.argoproj.io/tracking-id': 'my-app:apps/Deployment:default/my-deployment',
                    'some-other/annotation': 'keep-me'
                }
            }
        };

        const result = stripManagedFields(resource);

        expect(result.metadata.annotations['argocd.argoproj.io/tracking-id']).toBeUndefined();
        expect(result.metadata.annotations['some-other/annotation']).toBe('keep-me');
    });

    it('leaves annotations untouched when there is nothing to hide', () => {
        const resource = {
            metadata: {
                name: 'my-deployment',
                annotations: {'some-other/annotation': 'keep-me'}
            }
        };

        const result = stripManagedFields(resource);

        expect(result.metadata.annotations).toEqual({'some-other/annotation': 'keep-me'});
    });

    it('does not mutate the original resource', () => {
        const resource = {
            metadata: {
                name: 'my-deployment',
                managedFields: [{manager: 'argocd-controller'}],
                annotations: {'argocd.argoproj.io/tracking-id': 'my-app:apps/Deployment:default/my-deployment'}
            }
        };

        stripManagedFields(resource);

        expect(resource.metadata.managedFields).toBeDefined();
        expect(resource.metadata.annotations['argocd.argoproj.io/tracking-id']).toBeDefined();
    });

    it('returns the input unchanged when there is no metadata', () => {
        const resource = {kind: 'Deployment'};

        expect(stripManagedFields(resource)).toBe(resource);
    });
});
