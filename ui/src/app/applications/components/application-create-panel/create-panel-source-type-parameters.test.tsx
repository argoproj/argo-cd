import {Form, FormApi} from 'argo-ui';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as React from 'react';
import * as models from '../../../shared/models';
import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';
import {CreatePanelSourceTypeParameters} from './create-panel-source-type-parameters';

jest.mock('../../../shared/services', () => ({
    services: {
        authService: {
            settings: jest.fn().mockResolvedValue({kustomizeVersions: []})
        },
        repos: {
            appDetails: jest.fn()
        }
    }
}));

jest.mock('lodash-es', () => ({
    cloneDeep: (value: unknown) => JSON.parse(JSON.stringify(value))
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

function deferred<T>() {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>(res => (resolve = res));
    return {promise, resolve};
}

describe('CreatePanelSourceTypeParameters', () => {
    const appDetails = services.repos.appDetails as jest.Mock;

    beforeEach(() => {
        appDetails.mockReset();
    });

    test('stores a typed external Helm value file when Create is clicked without pressing Enter', async () => {
        const app = applicationWithSources([
            {repoURL: 'https://prometheus-community.github.io/helm-charts', chart: 'prometheus', targetRevision: '15.7.1'} as models.ApplicationSource,
            {repoURL: 'https://git.example.com/org/value-files.git', targetRevision: 'main', ref: 'values'} as models.ApplicationSource
        ]);
        appDetails.mockResolvedValue({
            type: 'Helm',
            path: '',
            helm: {name: 'prometheus', valueFiles: ['values.yaml'], parameters: [], fileParameters: []}
        } as models.RepoAppDetails);
        const onSubmit = jest.fn();
        const user = userEvent.setup();

        render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <Form defaultValues={app} onSubmit={onSubmit}>
                    {api => (
                        <>
                            <CreatePanelSourceTypeParameters formApi={api} sourceIndex={0} />
                            <button type='button' onClick={api.submitForm}>
                                Create
                            </button>
                        </>
                    )}
                </Form>
            </Context.Provider>
        );

        await screen.findByText('VALUES FILES');
        const valueFilesInput = document.querySelector('.tags-input input') as HTMLInputElement;
        expect(valueFilesInput).toHaveAttribute('placeholder', '$<ref>/path/to/values.yaml');
        const valueFile = '$values/charts/prometheus/values.yaml';
        fireEvent.change(valueFilesInput, {target: {value: valueFile}});
        await user.click(screen.getByRole('button', {name: 'Create'}));

        await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
        const submittedApp = onSubmit.mock.calls[0][0] as models.Application;
        expect(submittedApp.spec.sources?.[0].helm?.valueFiles).toEqual([valueFile]);
        expect(submittedApp.spec.sources?.[1].ref).toBe('values');
        expect(submittedApp.spec.sources?.[1].helm).toBeUndefined();
    });

    test('does not show generator parameters or discover a ref-only source', () => {
        const app = applicationWithSources([
            {repoURL: 'https://prometheus-community.github.io/helm-charts', chart: 'prometheus', targetRevision: '15.7.1'} as models.ApplicationSource,
            {repoURL: 'https://git.example.com/org/value-files.git', targetRevision: 'main', ref: 'values'} as models.ApplicationSource
        ]);

        const {container} = render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <Form defaultValues={app}>{api => <CreatePanelSourceTypeParameters formApi={api} sourceIndex={1} />}</Form>
            </Context.Provider>
        );

        expect(container).toBeEmptyDOMElement();
        expect(appDetails).not.toHaveBeenCalled();
    });

    test('does not carry an explicit generator type to another repository with the same path', async () => {
        const firstRepo = 'https://git.example.com/first.git';
        const secondRepo = 'https://git.example.com/second.git';
        const app = applicationWithSources([{repoURL: firstRepo, targetRevision: 'main', path: 'manifests'} as models.ApplicationSource]);
        appDetails.mockImplementation((source: models.ApplicationSource) =>
            Promise.resolve(
                source.repoURL === firstRepo
                    ? ({type: 'Directory', path: 'manifests', directory: {}} as models.RepoAppDetails)
                    : ({type: 'Directory', path: 'manifests', directory: {}} as models.RepoAppDetails)
            )
        );
        let formApi: FormApi | undefined;

        render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <Form defaultValues={app} getApi={api => (formApi = api)}>
                    {api => <CreatePanelSourceTypeParameters formApi={api} sourceIndex={0} />}
                </Form>
            </Context.Provider>
        );

        await screen.findByText('DIRECTORY');
        fireEvent.click(document.querySelector('[qe-id="application-create-dropdown-source-1-Kustomize"]') as HTMLElement);
        await screen.findByText('KUSTOMIZE');

        act(() => formApi!.setValue('spec.sources[0].repoURL', secondRepo));

        await screen.findByText('DIRECTORY');
        expect(screen.queryByText('KUSTOMIZE')).not.toBeInTheDocument();
    });

    test('does not carry an explicit generator type from a path source to a chart source', async () => {
        const repoURL = 'https://example.com/repository';
        const app = applicationWithSources([{repoURL, targetRevision: 'main', path: 'manifests'} as models.ApplicationSource]);
        appDetails.mockResolvedValue({type: 'Directory', path: 'manifests', directory: {}} as models.RepoAppDetails);
        let formApi: FormApi | undefined;

        render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <Form defaultValues={app} getApi={api => (formApi = api)}>
                    {api => <CreatePanelSourceTypeParameters formApi={api} sourceIndex={0} />}
                </Form>
            </Context.Provider>
        );

        await screen.findByText('DIRECTORY');
        fireEvent.click(document.querySelector('[qe-id="application-create-dropdown-source-1-Kustomize"]') as HTMLElement);
        await screen.findByText('KUSTOMIZE');

        const {path, ...sourceWithoutPath} = formApi!.values.spec.sources[0] as models.ApplicationSource;
        act(() => formApi!.setValue('spec.sources[0]', {...sourceWithoutPath, chart: path, targetRevision: '1.0.0'}));

        await screen.findByText('DIRECTORY');
        expect(screen.queryByText('KUSTOMIZE')).not.toBeInTheDocument();
    });

    test('ignores a stale discovery response from an intermediate repository shape', async () => {
        const helmRepo = 'https://charts.example.com';
        const gitRepo = 'https://git.example.com/application.git';
        const app = applicationWithSources([{repoURL: helmRepo, chart: 'application', targetRevision: '1.0.0'} as models.ApplicationSource]);
        const stale = deferred<models.RepoAppDetails>();
        const current = deferred<models.RepoAppDetails>();
        appDetails.mockImplementation((source: models.ApplicationSource) => {
            if (source.repoURL === helmRepo) {
                return Promise.resolve({type: 'Helm', path: '', helm: {name: 'application', valueFiles: [], parameters: [], fileParameters: []}} as models.RepoAppDetails);
            }
            return source.chart ? stale.promise : current.promise;
        });
        let formApi: FormApi | undefined;

        render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <Form defaultValues={app} getApi={api => (formApi = api)}>
                    {api => <CreatePanelSourceTypeParameters formApi={api} sourceIndex={0} />}
                </Form>
            </Context.Provider>
        );

        await screen.findByText('VALUES FILES');
        act(() => formApi!.setValue('spec.sources[0].repoURL', gitRepo));
        await waitFor(() => expect(appDetails.mock.calls.some(([source]) => source.repoURL === gitRepo && source.chart === 'application')).toBe(true));

        const {chart, ...sourceWithoutChart} = formApi!.values.spec.sources[0] as models.ApplicationSource;
        act(() => formApi!.setValue('spec.sources[0]', {...sourceWithoutChart, path: chart, targetRevision: 'HEAD'}));
        await waitFor(() => expect(appDetails.mock.calls.some(([source]) => source.repoURL === gitRepo && source.path === 'application')).toBe(true));

        await act(async () => current.resolve({type: 'Kustomize', path: 'application', kustomize: {path: 'application'}} as models.RepoAppDetails));
        await screen.findByText('KUSTOMIZE');
        await act(async () => stale.resolve({type: 'Directory', path: 'application', directory: {}} as models.RepoAppDetails));

        expect(screen.getByText('KUSTOMIZE')).toBeInTheDocument();
        expect(screen.queryByText('DIRECTORY')).not.toBeInTheDocument();
    });
});
