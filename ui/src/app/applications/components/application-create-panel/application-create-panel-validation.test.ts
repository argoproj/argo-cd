import * as models from '../../../shared/models';
import {validateApplicationCreate} from './application-create-panel-validation';

function multiSourceApplication(valuesSource: models.ApplicationSource): models.Application {
    return {
        metadata: {name: 'sandbox'},
        spec: {
            project: 'default',
            destination: {server: 'https://kubernetes.default.svc', namespace: 'sandbox'},
            sources: [
                {
                    repoURL: 'https://example.com/helm-charts',
                    chart: 'application',
                    targetRevision: '1.0.0',
                    helm: {valueFiles: ['$values/sandboxes/test-123/values.yaml'], parameters: [], fileParameters: []}
                },
                valuesSource
            ]
        }
    } as models.Application;
}

describe('validateApplicationCreate', () => {
    test('accepts a ref-only source for Helm value files', () => {
        const app = multiSourceApplication({
            repoURL: 'https://git.example.com/infrastructure/values.git',
            targetRevision: 'main',
            ref: 'values'
        } as models.ApplicationSource);

        const errors = validateApplicationCreate(app, true);

        expect(errors['spec.sources[1].path']).toBeUndefined();
        expect(errors['spec.sources[1].chart']).toBeUndefined();
        expect(Object.values(errors).filter(Boolean)).toEqual([]);
    });

    test('rejects a source without a path, chart, or ref', () => {
        const app = multiSourceApplication({
            repoURL: 'https://git.example.com/infrastructure/values.git',
            targetRevision: 'main'
        } as models.ApplicationSource);

        const errors = validateApplicationCreate(app, true);

        expect(errors['spec.sources[1].path']).toBe('Path or Ref is required');
        expect(errors['spec.sources[1].chart']).toBe('Chart is required');
    });

    test('still requires a repository URL for a ref-only source', () => {
        const app = multiSourceApplication({targetRevision: 'main', ref: 'values'} as models.ApplicationSource);

        const errors = validateApplicationCreate(app, true);

        expect(errors['spec.sources[1].repoURL']).toBe('Repository URL is required');
    });
});
