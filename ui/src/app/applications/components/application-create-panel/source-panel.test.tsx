import {fireEvent, render, screen} from '@testing-library/react';
import {Form, FormApi} from 'argo-ui';
import * as React from 'react';

import * as models from '../../../shared/models';
import {SourcePanel} from './source-panel';

// lodash-es ships as untransformed ESM under node_modules and is not covered by the jest
// transform config, so map the single helper this module uses to a CJS-friendly stub.
jest.mock('lodash-es', () => ({
    __esModule: true,
    cloneDeep: (value: any) => (value == null ? value : JSON.parse(JSON.stringify(value)))
}));

// The panel prefetches revisions and paths for the repository it is showing; keep it offline.
jest.mock('../../../shared/services', () => ({
    services: {
        repos: {
            apps: jest.fn().mockResolvedValue([]),
            charts: jest.fn().mockResolvedValue([]),
            revisions: jest.fn().mockResolvedValue({branches: [], tags: []}),
            ociTags: jest.fn().mockResolvedValue({tags: []})
        }
    }
}));

// Switching the repository type only touches the source object; the panel then needs a re-render.
// argo-ui's Form re-renders its children only when it is handed a new values object, so this test
// asserts that the source type is switched through `setAllValues` with a new object and that the
// fields rendered afterwards belong to the new type.

const gitApp = (): models.Application =>
    ({
        metadata: {name: 'test-app'},
        spec: {
            project: 'default',
            destination: {server: 'https://kubernetes.default.svc', namespace: 'default'},
            source: {repoURL: 'https://github.com/example/repo', targetRevision: 'HEAD', path: 'manifests'}
        }
    }) as models.Application;

let formApi: FormApi;

const Harness = () => {
    const currentRepoType = React.useRef<string | undefined>(undefined);
    const lastGitOrHelmUrl = React.useRef('');
    const lastOciUrl = React.useRef('');
    return (
        <Form defaultValues={gitApp()} getApi={(api: FormApi) => (formApi = api)}>
            {(api: FormApi) => <SourcePanel formApi={api} repos={[]} currentRepoType={currentRepoType} lastGitOrHelmUrl={lastGitOrHelmUrl} lastOciUrl={lastOciUrl} />}
        </Form>
    );
};

const switchRepoType = (to: string) => {
    fireEvent.click(document.querySelector('[qe-id="application-create-dropdown-source-repository"]'));
    fireEvent.click(document.querySelector(`[qe-id="application-create-dropdown-source-repository-${to}"]`));
};

describe('SourcePanel repository type switch', () => {
    beforeEach(() => {
        render(<Harness />);
    });

    test('re-renders the Git fields as Helm fields', async () => {
        expect(await screen.findByText('Path')).toBeInTheDocument();

        switchRepoType('HELM');

        expect(await screen.findByText('Chart')).toBeInTheDocument();
        expect(screen.queryByText('Path')).not.toBeInTheDocument();
        expect(formApi.getFormState().values.spec.source).toEqual(expect.objectContaining({chart: 'manifests', targetRevision: ''}));
        expect(formApi.getFormState().values.spec.source.path).toBeUndefined();
    });

    test('re-renders the Helm fields as Git fields', async () => {
        switchRepoType('HELM');
        switchRepoType('GIT');

        expect(await screen.findByText('Path')).toBeInTheDocument();
        expect(screen.queryByText('Chart')).not.toBeInTheDocument();
        expect(formApi.getFormState().values.spec.source).toEqual(expect.objectContaining({path: 'manifests', targetRevision: 'HEAD'}));
        expect(formApi.getFormState().values.spec.source.chart).toBeUndefined();
    });
});
