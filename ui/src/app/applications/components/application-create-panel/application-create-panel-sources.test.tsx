import {FormApi} from 'argo-ui';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as React from 'react';
import * as models from '../../../shared/models';
import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';
import {ApplicationCreatePanel} from './application-create-panel';

jest.mock('../../../shared/services', () => ({
    services: {
        authService: {
            settings: jest.fn().mockResolvedValue({kustomizeVersions: []}),
            plugins: jest.fn().mockResolvedValue([])
        },
        projects: {
            list: jest.fn().mockResolvedValue([{metadata: {name: 'default'}}])
        },
        clusters: {
            list: jest.fn().mockResolvedValue([])
        },
        repos: {
            list: jest.fn().mockResolvedValue([]),
            apps: jest.fn().mockResolvedValue([]),
            appDetails: jest.fn()
        }
    }
}));

jest.mock('lodash-es', () => ({
    cloneDeep: (value: unknown) => JSON.parse(JSON.stringify(value)),
    debounce: (callback: (...args: any[]) => void) => Object.assign(callback, {cancel: jest.fn()})
}));

jest.mock('../../../shared/components/yaml-editor/yaml-editor', () => ({YamlEditor: () => null}));
jest.mock('../revision-form-field/revision-form-field', () => ({RevisionFormField: () => null}));

test.each([false, true])('keeps configured generator settings visible after changing repositories (multi-source: %s)', async multiSource => {
    const firstRepo = 'https://git.example.com/first.git';
    const secondRepo = 'https://git.example.com/second.git';
    const source = {repoURL: firstRepo, targetRevision: 'main', path: 'manifests'};
    const valuesSource = {repoURL: 'https://git.example.com/values.git', targetRevision: 'main', ref: 'values'};
    const app = {
        metadata: {name: 'sandbox'},
        spec: {
            project: 'default',
            destination: {server: 'https://kubernetes.default.svc', namespace: 'sandbox'},
            ...(multiSource ? {sources: [source, valuesSource]} : {source})
        }
    } as models.Application;
    const appDetails = services.repos.appDetails as jest.Mock;
    appDetails.mockReset().mockResolvedValue({type: 'Directory', path: 'manifests', directory: {}});
    const createApp = jest.fn();
    let formApi: FormApi;
    const getSource = () => (multiSource ? formApi.values.spec.sources[0] : formApi.values.spec.source) as models.ApplicationSource;
    const dropdown = multiSource ? 'application-create-dropdown-source-1' : 'application-create-dropdown-source';
    const repoField = multiSource ? 'application-create-source-1-field-repository-url' : 'application-create-field-repository-url';

    render(
        <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
            <ApplicationCreatePanel app={app} onAppChanged={jest.fn()} createApp={createApp} getFormApi={api => (formApi = api)} />
        </Context.Provider>
    );

    await screen.findByText('DIRECTORY');
    fireEvent.click(document.querySelector(`[qe-id="${dropdown}-Kustomize"]`) as HTMLElement);
    const prefixRow = (await screen.findByText('NAME PREFIX')).closest('.white-box__details-row');
    fireEvent.change(prefixRow.querySelector('input'), {target: {value: 'custom-'}});
    await waitFor(() => expect(getSource().kustomize?.namePrefix).toBe('custom-'));

    fireEvent.change(document.querySelector(`[qe-id="${repoField}"]`), {target: {value: secondRepo}});

    await waitFor(() => expect(appDetails.mock.calls.some(([requestedSource]) => requestedSource.repoURL === secondRepo)).toBe(true));
    expect(await screen.findByText('KUSTOMIZE')).toBeInTheDocument();
    expect(screen.getByDisplayValue('custom-')).toBeInTheDocument();
    expect(screen.queryByText('DIRECTORY RECURSE')).not.toBeInTheDocument();
    expect(getSource().kustomize).toEqual({namePrefix: 'custom-'});

    fireEvent.click(document.querySelector(`[qe-id="${dropdown}-Directory"]`) as HTMLElement);
    const recurseRow = (await screen.findByText('DIRECTORY RECURSE')).closest('.white-box__details-row');
    fireEvent.click(recurseRow.querySelector('input'));
    await waitFor(() => expect(getSource().directory?.recurse).toBe(true));

    await act(async () => formApi.submitForm());

    expect(createApp).toHaveBeenCalledTimes(1);
    const submitted = createApp.mock.calls[0][0] as models.Application;
    expect(multiSource ? submitted.spec.sources[0] : submitted.spec.source).toEqual({
        repoURL: secondRepo,
        targetRevision: 'main',
        path: 'manifests',
        directory: {recurse: true}
    });
    if (multiSource) {
        expect(submitted.spec.sources[1]).toEqual(valuesSource);
    }
});

const generatorConfigurations = [
    {type: 'Helm', settings: {helm: {valueFiles: ['$values/production.yaml'], releaseName: 'web'}}},
    {type: 'Kustomize', settings: {kustomize: {namePrefix: 'production-'}}},
    {type: 'Directory', settings: {directory: {recurse: true, include: '*.yaml'}}},
    {type: 'Plugin', settings: {plugin: {name: 'custom', env: [{name: 'ENV', value: 'production'}]}}}
];

test.each(generatorConfigurations)('preserves $type settings while replacing a Git path with Ref', async ({type, settings}) => {
    const source = {repoURL: 'https://git.example.com/app.git', targetRevision: 'main', path: 'old-path', ref: 'app', ...settings};
    const valuesSource = {repoURL: 'https://git.example.com/values.git', targetRevision: 'main', ref: 'values'};
    const app = {
        metadata: {name: 'sandbox'},
        spec: {project: 'default', destination: {server: 'https://kubernetes.default.svc', namespace: 'sandbox'}, sources: [source, valuesSource]}
    } as models.Application;
    (services.repos.appDetails as jest.Mock).mockReset().mockResolvedValue({type: 'Directory', directory: {}});
    const createApp = jest.fn();
    const user = userEvent.setup();
    let formApi: FormApi;
    render(
        <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
            <ApplicationCreatePanel app={app} onAppChanged={jest.fn()} createApp={createApp} getFormApi={api => (formApi = api)} />
        </Context.Provider>
    );

    await screen.findByText(type.toUpperCase(), {selector: 'p'});
    const path = await screen.findByDisplayValue('old-path');
    await user.clear(path);
    await user.tab();
    expect(formApi.values.spec.sources[0]).toEqual({...source, path: ''});
    expect(screen.queryByText(type.toUpperCase(), {selector: 'p'})).not.toBeInTheDocument();

    await user.type(path, 'new-path');
    await screen.findByText(type.toUpperCase(), {selector: 'p'});
    await act(async () => formApi.submitForm());

    expect(createApp).toHaveBeenCalledTimes(1);
    expect(createApp.mock.calls[0][0].spec.sources).toEqual([{...source, path: 'new-path'}, valuesSource]);
});

test.each(generatorConfigurations)('removes $type settings only from the submitted ref-only source', async ({type, settings}) => {
    const source = {repoURL: 'https://git.example.com/app.git', targetRevision: 'main', path: 'old-path', ref: 'app', ...settings};
    const chartSource = {
        repoURL: 'https://git.example.com/chart.git',
        targetRevision: 'main',
        path: 'chart',
        helm: {valueFiles: ['$app/production.yaml']}
    };
    const app = {
        metadata: {name: 'sandbox'},
        spec: {project: 'default', destination: {server: 'https://kubernetes.default.svc', namespace: 'sandbox'}, sources: [source, chartSource]}
    } as models.Application;
    (services.repos.appDetails as jest.Mock).mockReset().mockResolvedValue({type: 'Directory', directory: {}});
    const createApp = jest.fn();
    const user = userEvent.setup();
    let formApi: FormApi;
    render(
        <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
            <ApplicationCreatePanel app={app} onAppChanged={jest.fn()} createApp={createApp} getFormApi={api => (formApi = api)} />
        </Context.Provider>
    );

    await screen.findAllByText(type.toUpperCase(), {selector: 'p'});
    await user.clear(await screen.findByDisplayValue('old-path'));
    await act(async () => formApi.submitForm());

    expect(createApp).toHaveBeenCalledTimes(1);
    expect(createApp.mock.calls[0][0].spec.sources).toEqual([{repoURL: source.repoURL, targetRevision: 'main', path: '', ref: 'app'}, chartSource]);
    // A failed creation must leave the draft available for further editing.
    expect(formApi.values.spec.sources[0]).toEqual({...source, path: ''});
});
