import {Form, FormApi} from 'argo-ui';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import * as React from 'react';
import * as models from '../../../shared/models';
import {SourcePanel} from './source-panel';

jest.mock('../../../shared/services', () => ({
    services: {
        repos: {
            apps: jest.fn().mockResolvedValue([]),
            charts: jest.fn().mockResolvedValue([])
        }
    }
}));

jest.mock('../revision-form-field/revision-form-field', () => ({
    RevisionFormField: () => null
}));

jest.mock('../../../shared/components', () => ({
    RevisionHelpIcon: () => null
}));

function applicationWithSources(sources: models.ApplicationSource[]): models.Application {
    return {
        metadata: {name: 'sandbox'},
        spec: {
            project: 'default',
            destination: {server: 'https://kubernetes.default.svc', namespace: 'sandbox'},
            sources
        }
    } as models.Application;
}

describe('SourcePanel', () => {
    test('binds Ref to the selected Git source in a multi-source application', async () => {
        const app = applicationWithSources([
            {repoURL: 'https://example.com/helm-charts', chart: 'application', targetRevision: '1.0.0'} as models.ApplicationSource,
            {repoURL: 'https://git.example.com/infrastructure/values.git', targetRevision: 'main'} as models.ApplicationSource
        ]);
        let formApi: FormApi | undefined;

        render(
            <Form defaultValues={app} getApi={api => (formApi = api)}>
                {api => (
                    <SourcePanel
                        formApi={api}
                        repos={[]}
                        repoInfo={{repo: 'https://git.example.com/infrastructure/values.git', type: 'git'} as models.Repository}
                        sourceIndex={1}
                    />
                )}
            </Form>
        );

        await waitFor(() => expect(document.querySelector('[qe-id="application-create-source-2-field-path"]')).not.toBeNull());
        const refInput = screen.getByRole('textbox', {name: 'Ref'});
        fireEvent.change(refInput, {target: {value: 'values'}});

        expect(formApi?.values.spec.sources[1].ref).toBe('values');
        expect(formApi?.values.spec.sources[0].ref).toBeUndefined();
    });

    test('does not offer Ref for a Helm source', async () => {
        const app = applicationWithSources([{repoURL: 'https://example.com/helm-charts', chart: 'application', targetRevision: '1.0.0'} as models.ApplicationSource]);

        render(
            <Form defaultValues={app}>
                {api => <SourcePanel formApi={api} repos={[]} repoInfo={{repo: 'https://example.com/helm-charts', type: 'helm'} as models.Repository} sourceIndex={0} />}
            </Form>
        );

        await screen.findByDisplayValue('application');
        expect(screen.queryByRole('textbox', {name: 'Ref'})).toBeNull();
    });

    test.each([
        ['HELM', ''],
        ['OCI', undefined]
    ])('clears Ref when changing a ref-only Git source to %s', async (targetType, expectedChart) => {
        const app = applicationWithSources([
            {
                repoURL: 'https://git.example.com/infrastructure/values.git',
                targetRevision: 'main',
                ref: 'values'
            } as models.ApplicationSource
        ]);
        let formApi: FormApi | undefined;

        render(
            <Form defaultValues={app} getApi={api => (formApi = api)}>
                {api => <SourcePanel formApi={api} repos={[]} sourceIndex={0} />}
            </Form>
        );

        await screen.findByRole('textbox', {name: 'Ref'});
        fireEvent.click(document.querySelector(`[qe-id="application-create-dropdown-source-repository-1-${targetType}"]`) as HTMLElement);

        await waitFor(() => expect(formApi?.values.spec.sources[0].ref).toBeUndefined());
        expect(formApi?.values.spec.sources[0].chart).toBe(expectedChart);
        expect(formApi?.values.spec.sources[0].repoURL).toBe(targetType === 'OCI' ? 'oci://' : 'https://git.example.com/infrastructure/values.git');
    });
});
