import {act, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as React from 'react';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {Context} from '../../../shared/context';
import {ApplicationParameters} from './application-parameters';

jest.mock('../../../shared/services', () => ({
    services: {
        repos: {
            appDetails: jest.fn()
        },
        authService: {
            plugins: jest.fn().mockResolvedValue([{name: 'my-plugin'}])
        },
        viewPreferences: {
            getPreferences: jest.fn().mockResolvedValue({pageSizes: {'5': 10}, sortOptions: {}}),
            updatePreferences: jest.fn()
        }
    }
}));

jest.mock('lodash-es', () => ({
    cloneDeep: (value: unknown) => JSON.parse(JSON.stringify(value))
}));

jest.mock('./source-panel', () => ({
    SourcePanel: () => null
}));

jest.mock('../utils', () => ({
    deleteSourceAction: jest.fn(),
    getAppDefaultSource: (app: models.Application) => app.spec.source || app.spec.sources?.[0],
    getAppDrySource: (app: models.Application) => app.spec.source,
    helpTip: () => null
}));

function applicationWithValuesSource(valuesSource: models.ApplicationSource): models.Application {
    return {
        metadata: {name: 'sandbox'},
        spec: {
            project: 'default',
            destination: {server: 'https://kubernetes.default.svc', namespace: 'sandbox'},
            sources: [{repoURL: 'https://prometheus-community.github.io/helm-charts', chart: 'prometheus', targetRevision: '15.7.1'} as models.ApplicationSource, valuesSource]
        }
    } as models.Application;
}

function renderParameters(app: models.Application) {
    return render(
        <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
            <ApplicationParameters application={app} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[true, false]} handleCollapse={jest.fn()} appContext={{} as any} />
        </Context.Provider>
    );
}

describe('ApplicationParameters ref-only sources', () => {
    const appDetails = services.repos.appDetails as jest.Mock;

    beforeEach(() => {
        appDetails.mockReset();
    });

    test.each([
        [{ref: 'values'}, 'REF=values, REVISION=main'],
        [{ref: 'values', path: 'manifests'}, 'PATH=manifests, REF=values, REVISION=main'],
        [{path: 'manifests'}, 'PATH=manifests, REVISION=main']
    ])('summarizes the collapsed source %j', async (source, summary) => {
        const app = applicationWithValuesSource({
            repoURL: 'https://git.example.com/org/value-files.git',
            targetRevision: 'main',
            ...source
        } as models.ApplicationSource);

        renderWithContext(
            <ApplicationParameters application={app} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[true, true]} handleCollapse={jest.fn()} />
        );

        expect(await screen.findByText(summary)).toBeInTheDocument();
        expect(screen.getByText('CHART=prometheus, REVISION=15.7.1')).toBeInTheDocument();
        expect(appDetails).not.toHaveBeenCalled();
    });

    test('shows only core fields without running source discovery', async () => {
        renderParameters(
            applicationWithValuesSource({
                repoURL: 'https://git.example.com/org/value-files.git',
                targetRevision: 'main',
                ref: 'values'
            } as models.ApplicationSource)
        );

        expect(await screen.findByText('Source 2: REF=values, URL=https://git.example.com/org/value-files.git, TARGET REVISION=main')).toBeInTheDocument();
        expect(screen.getByText('REF')).toBeInTheDocument();
        expect(screen.getByText('values')).toBeInTheDocument();
        expect(screen.queryByText('PLUGIN')).not.toBeInTheDocument();
        expect(screen.queryByText('DIRECTORY')).not.toBeInTheDocument();
        expect(appDetails).not.toHaveBeenCalled();
    });

    test('still discovers a source that has both ref and path', async () => {
        appDetails.mockResolvedValue({type: 'Plugin', path: 'manifests', plugin: {name: 'example', env: []}} as models.RepoAppDetails);
        const source = {
            repoURL: 'https://git.example.com/org/value-files.git',
            targetRevision: 'main',
            path: 'manifests',
            ref: 'values'
        } as models.ApplicationSource;

        renderParameters(applicationWithValuesSource(source));

        await waitFor(() => expect(appDetails).toHaveBeenCalledWith(source, 'sandbox', 'default', 1, 0));
        expect(await screen.findByText('PLUGIN')).toBeInTheDocument();
        expect(screen.getByText(/Source 2: TYPE=Plugin/)).toBeInTheDocument();
    });

    test('makes external values guidance accessible and saves typed files without Enter', async () => {
        const app = applicationWithValuesSource({
            repoURL: 'https://git.example.com/org/value-files.git',
            targetRevision: 'main',
            ref: 'values'
        } as models.ApplicationSource);
        app.spec.sources[0].helm = {valueFiles: ['$values/charts/prometheus/values.yaml']} as models.ApplicationSourceHelm;
        appDetails.mockResolvedValue({
            type: 'Helm',
            helm: {name: 'prometheus', valueFiles: ['values.yaml'], parameters: [], fileParameters: []}
        } as models.RepoAppDetails);
        const user = userEvent.setup();
        const save = jest.fn().mockResolvedValue(undefined);

        render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <ApplicationParameters
                    application={app}
                    save={save}
                    pageNumber={0}
                    setPageNumber={jest.fn()}
                    collapsedSources={[false, true]}
                    handleCollapse={jest.fn()}
                    appContext={{} as any}
                />
            </Context.Provider>
        );

        const help = await screen.findByRole('button', {name: 'Help for VALUES FILES'});
        expect(help).toHaveAttribute('type', 'button');

        const viewTitle = await screen.findByText('VALUES FILES');
        expect(viewTitle.closest('.columns.small-3')?.querySelector('.help-tip')).not.toBeNull();

        const editButtons = screen.getAllByRole('button', {name: 'Edit'});
        await user.click(editButtons[1]);

        await waitFor(() => expect(screen.getByText('VALUES FILES').closest('.columns.small-3')?.querySelector('.help-tip')).not.toBeNull());
        const input = screen.getByRole('combobox', {name: 'Values files'});
        expect(input).toHaveAccessibleDescription(
            'To use a file from another Git source, set its Ref and enter $<ref>/path/to/values.yaml. The path is relative to that repository root.'
        );
        await user.type(input, '$values/environments/stage.yaml');
        const helmSection = screen.getByText('HELM').parentElement.parentElement;
        await user.click(within(helmSection).getByRole('button', {name: 'Save', exact: true}));
        await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
        expect(save.mock.calls[0][0].spec.sources[0].helm.valueFiles).toEqual(['$values/charts/prometheus/values.yaml', '$values/environments/stage.yaml']);
    });

    test.each([
        ['repository details cannot be loaded', () => appDetails.mockRejectedValue(new Error('failed to load application details'))],
        ['repository details report a different type', () => appDetails.mockResolvedValue({type: 'Directory', path: 'charts/web-app'} as models.RepoAppDetails)]
    ])('uses the explicit Helm source when %s', async (_name, arrangeAppDetails) => {
        arrangeAppDetails();
        const app = applicationWithValuesSource({
            repoURL: 'https://git.example.com/org/value-files.git',
            targetRevision: 'main',
            ref: 'values'
        } as models.ApplicationSource);
        app.spec.sources[0] = {
            repoURL: 'https://git.example.com/org/charts.git',
            targetRevision: 'main',
            path: 'charts/web-app',
            helm: {valueFiles: ['$values/sandboxes/multisource-ui-test/values.yaml']}
        } as models.ApplicationSource;

        render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <ApplicationParameters
                    application={app}
                    pageNumber={0}
                    setPageNumber={jest.fn()}
                    collapsedSources={[false, true]}
                    handleCollapse={jest.fn()}
                    appContext={{} as any}
                />
            </Context.Provider>
        );

        expect(await screen.findByText(/Source 1: TYPE=Helm/)).toBeInTheDocument();
        expect(screen.getByText('HELM')).toBeInTheDocument();
        expect(screen.getByText('$values/sandboxes/multisource-ui-test/values.yaml')).toBeInTheDocument();
        expect(screen.getByText('VALUES FILES').closest('.columns.small-3')?.querySelector('.help-tip')).not.toBeNull();
        expect(screen.queryByText('DIRECTORY')).not.toBeInTheDocument();
    });
});

const multiSourceApp = (): models.Application =>
    ({
        metadata: {name: 'test-app'},
        spec: {
            project: 'default',
            sources: [
                {
                    repoURL: 'https://github.com/example/repo',
                    targetRevision: 'main',
                    plugin: {
                        name: 'my-plugin',
                        env: [
                            {name: 'FOO', value: 'bar'},
                            {name: 'ENVIRONMENT', value: 'prod'}
                        ]
                    }
                }
            ]
        }
    }) as models.Application;

// EditablePanel (rendered by the expanded panel) reads notifications off the app Context,
// so provide a minimal Context value.
const ctxValue = {notifications: {show: () => undefined}} as any;
const renderWithContext = (element: React.ReactElement) => render(<Context.Provider value={ctxValue}>{element}</Context.Provider>);

function deferredRepoDetails() {
    let resolve: (details: models.RepoAppDetails) => void;
    let reject: (error: Error) => void;
    const promise = new Promise<models.RepoAppDetails>((onResolve, onReject) => {
        resolve = onResolve;
        reject = onReject;
    });
    return {promise, resolve, reject};
}

test('collapsed multi-source summary shows plugin env values', async () => {
    renderWithContext(<ApplicationParameters application={multiSourceApp()} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[true]} handleCollapse={() => undefined} />);

    expect(await screen.findByText(/ENV=\[FOO=bar, ENVIRONMENT=prod\]/)).toBeInTheDocument();
});

test('expanded multi-source plugin panel renders NAME and ENV from the per-source plugin', async () => {
    (services.repos.appDetails as jest.Mock).mockResolvedValue({type: 'Plugin', plugin: {}});
    // collapsedSources[0] = false -> the expanded panel renders and reads spec.sources[0].plugin.
    // Before the fix it read the (absent) single-source spec.source.plugin, so NAME/ENV were blank.
    renderWithContext(<ApplicationParameters application={multiSourceApp()} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[false]} handleCollapse={() => undefined} />);

    // NAME and ENV are rendered as read-only inputs (see NameValueEditor/ValueEditor), so assert by display value.
    expect(await screen.findByDisplayValue('my-plugin')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('FOO')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('bar')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('ENVIRONMENT')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('prod')).toBeInTheDocument();
});

test('multi-source default (no collapse state) renders the expanded panel, not the raw-text summary', async () => {
    // The details page initializes collapsedSources as an empty array, so every entry is undefined (see argoproj/argo-cd#30942).
    (services.repos.appDetails as jest.Mock).mockResolvedValue({type: 'Plugin', plugin: {}});
    renderWithContext(<ApplicationParameters application={multiSourceApp()} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[]} handleCollapse={() => undefined} />);

    expect(await screen.findByDisplayValue('my-plugin')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('FOO')).toBeInTheDocument();
    expect(screen.queryByText(/ENV=\[FOO=bar, ENVIRONMENT=prod\]/)).not.toBeInTheDocument();
});

describe.each([false, true])('ApplicationParameters repository failures (multi-source: %s)', multiSource => {
    const appDetails = services.repos.appDetails as jest.Mock;
    const warning = 'Unable to load repository details. Showing configured source settings';

    beforeEach(() => {
        appDetails.mockReset();
    });

    test.each([
        {
            type: 'Helm',
            source: {
                chart: 'web-app',
                helm: {valueFiles: ['production.yaml'], parameters: [{name: 'replicas', value: '3'}]}
            },
            assertSettings: () => {
                expect(screen.getByText('production.yaml')).toBeInTheDocument();
                expect(screen.getByText('replicas')).toBeInTheDocument();
                expect(screen.getByText('3')).toBeInTheDocument();
            }
        },
        {
            type: 'Plugin',
            source: {
                path: 'manifests',
                plugin: {name: 'my-plugin', env: [{name: 'FOO', value: 'bar'}], parameters: [{name: 'replicas', string: '3'}]}
            },
            assertSettings: () => {
                expect(screen.getByDisplayValue('my-plugin')).toBeInTheDocument();
                expect(screen.getByDisplayValue('FOO')).toBeInTheDocument();
                expect(screen.getByDisplayValue('bar')).toBeInTheDocument();
                expect(screen.getByText('replicas')).toBeInTheDocument();
                expect(screen.getByDisplayValue('3')).toBeInTheDocument();
            }
        }
    ])('warns without hiding configured $type settings', async ({type, source, assertSettings}) => {
        const appSource = {repoURL: 'https://github.com/example/repo', targetRevision: 'main', ...source} as models.ApplicationSource;
        const app = multiSourceApp();
        if (multiSource) {
            app.spec.sources = [appSource];
        } else {
            app.spec.source = appSource;
            delete app.spec.sources;
        }
        appDetails.mockRejectedValue(new Error('repository unavailable'));

        const {rerender} = renderWithContext(<ApplicationParameters application={app} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[]} handleCollapse={jest.fn()} />);

        expect(await screen.findByRole('alert')).toHaveTextContent(`${warning}: repository unavailable`);
        expect(screen.getByText(type.toUpperCase())).toBeInTheDocument();
        expect(screen.queryByText('DIRECTORY')).not.toBeInTheDocument();
        assertSettings();
        if (multiSource) {
            expect(screen.getByText(appSource.repoURL)).toBeInTheDocument();
            expect(screen.getByText(type === 'Helm' ? 'web-app:main' : 'manifests')).toBeInTheDocument();
        }

        appDetails.mockResolvedValue(
            type === 'Helm' ? {type: 'Helm', helm: {valueFiles: [], parameters: [], fileParameters: []}} : {type: 'Plugin', path: 'manifests', plugin: {}}
        );
        const updatedSource = {...appSource, targetRevision: 'release'};
        const updatedApp = {...app, spec: {...app.spec, ...(multiSource ? {sources: [updatedSource]} : {source: updatedSource})}};
        rerender(
            <Context.Provider value={ctxValue}>
                <ApplicationParameters application={updatedApp} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[]} handleCollapse={jest.fn()} />
            </Context.Provider>
        );

        await waitFor(() => expect(appDetails).toHaveBeenLastCalledWith(updatedSource, 'test-app', 'default', 0, 0));
        await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument());
        expect(screen.getByText(type.toUpperCase())).toBeInTheDocument();
        assertSettings();
    });

    test('warns when the source type cannot be discovered', async () => {
        const appSource = {repoURL: 'https://github.com/example/repo', path: 'manifests', targetRevision: 'main'} as models.ApplicationSource;
        const app = multiSourceApp();
        if (multiSource) {
            app.spec.sources = [appSource];
        } else {
            app.spec.source = appSource;
            delete app.spec.sources;
        }
        appDetails.mockRejectedValue(new Error('repository unavailable'));

        const parameters = (source: models.ApplicationSource) => (
            <Context.Provider value={ctxValue}>
                <ApplicationParameters application={{...app, spec: {...app.spec, ...(multiSource ? {sources: [source]} : {source})}}} pageNumber={0} setPageNumber={jest.fn()} />
            </Context.Provider>
        );
        const {rerender} = render(parameters(appSource));

        expect(await screen.findByRole('alert')).toHaveTextContent('Source type unknown');
        expect(screen.queryByText('DIRECTORY')).not.toBeInTheDocument();
        expect(screen.queryByText('DIRECTORY RECURSE')).not.toBeInTheDocument();
        expect(screen.queryByText(/TYPE=Directory/)).not.toBeInTheDocument();
        expect(appDetails).toHaveBeenCalledWith(appSource, 'test-app', 'default', 0, 0);
        if (multiSource) {
            expect(screen.getByText(appSource.repoURL)).toBeInTheDocument();
            expect(screen.getByText('manifests')).toBeInTheDocument();
            expect(screen.getByText('main')).toBeInTheDocument();
        }

        rerender(parameters({...appSource, targetRevision: 'explicit', directory: {recurse: true}}));
        expect(await screen.findByText('DIRECTORY')).toBeInTheDocument();
        expect(screen.getByRole('alert')).toHaveTextContent(warning);
        expect(screen.queryByText(/Source type unknown/)).not.toBeInTheDocument();

        appDetails.mockResolvedValue({type: 'Helm', helm: {parameters: [{name: 'recovered-repository-default', value: 'ready'}]}} as models.RepoAppDetails);
        rerender(parameters({...appSource, targetRevision: 'release'}));
        expect(await screen.findByText('recovered-repository-default')).toBeInTheDocument();
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();

        appDetails.mockResolvedValue({type: 'Directory', path: 'manifests', directory: {}} as models.RepoAppDetails);
        rerender(parameters({...appSource, targetRevision: 'directory-discovery'}));
        expect(await screen.findByText('DIRECTORY')).toBeInTheDocument();
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    test('retries a failed lookup without changing its input', async () => {
        const app = multiSourceApp();
        const appSource = app.spec.sources[0];
        appSource.plugin.parameters = [{name: 'options', string: 'configured'}];
        if (!multiSource) {
            app.spec.source = appSource;
            delete app.spec.sources;
        }
        appDetails.mockRejectedValueOnce(new Error('repository unavailable')).mockResolvedValue({
            type: 'Plugin',
            plugin: {
                parametersAnnouncement: [
                    {name: 'options', collectionType: 'string'},
                    {name: 'retry-default', string: 'ready'}
                ]
            }
        });
        const user = userEvent.setup();
        renderWithContext(<ApplicationParameters application={app} save={jest.fn()} pageNumber={0} setPageNumber={jest.fn()} />);

        expect(await screen.findByRole('alert')).toHaveTextContent('repository unavailable');
        expect(screen.getByDisplayValue('my-plugin')).toBeInTheDocument();
        await user.click(screen.getAllByRole('button', {name: 'Edit'})[multiSource ? 1 : 0]);
        const editor = screen.getByDisplayValue('configured');
        await user.clear(editor);
        await user.type(editor, 'user-draft');
        await user.click(screen.getByRole('button', {name: 'Retry'}));
        expect(await screen.findByText('retry-default')).toBeInTheDocument();
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        expect(screen.getByDisplayValue('user-draft')).toBe(editor);
        expect(appDetails).toHaveBeenCalledTimes(2);
        expect(appDetails.mock.calls[0]).toEqual([appSource, 'test-app', 'default', 0, 0]);
        expect(appDetails.mock.calls[1]).toEqual(appDetails.mock.calls[0]);
    });

    test('preserves a Helm valuesObject draft across watch updates', async () => {
        const app = multiSourceApp();
        const source = {
            repoURL: 'https://charts.example.com',
            chart: 'web-app',
            targetRevision: '1.0.0',
            helm: {valuesObject: {replicaCount: 1}}
        } as models.ApplicationSource;
        if (multiSource) {
            app.spec.sources = [source];
        } else {
            app.spec.source = source;
            delete app.spec.sources;
        }
        appDetails.mockResolvedValue({type: 'Helm', helm: {parameters: [], valueFiles: [], fileParameters: []}});
        const save = jest.fn();
        const parameters = (version: string) => (
            <Context.Provider value={ctxValue}>
                <ApplicationParameters application={{...app, metadata: {...app.metadata, resourceVersion: version}}} save={save} pageNumber={0} setPageNumber={jest.fn()} />
            </Context.Provider>
        );
        const user = userEvent.setup();
        const {container, rerender} = render(parameters('1'));
        expect(await screen.findByText('HELM')).toBeInTheDocument();
        await user.click(screen.getAllByRole('button', {name: 'Edit'})[multiSource ? 1 : 0]);
        const editor = container.querySelector('textarea');
        await user.clear(editor);
        await user.type(editor, 'replicaCount: 7\n');
        expect(editor).toHaveValue('replicaCount: 7\n');

        rerender(parameters('2'));

        expect(appDetails).toHaveBeenCalledTimes(1);
        expect(container.querySelector('textarea')).toBe(editor);
        expect(editor).toHaveValue('replicaCount: 7\n');
        expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
        expect(save).not.toHaveBeenCalled();
    });

    test.each([false, true])('preserves an unsaved Helm draft during Retry (success: %s)', async retrySuccess => {
        const app = multiSourceApp();
        const source = {
            repoURL: 'https://charts.example.com',
            chart: 'web-app',
            targetRevision: '1.0.0',
            helm: {valuesObject: {replicaCount: 1}}
        } as models.ApplicationSource;
        if (multiSource) {
            app.spec.sources = [source];
        } else {
            app.spec.source = source;
            delete app.spec.sources;
        }
        const request = deferredRepoDetails();
        appDetails.mockRejectedValueOnce(new Error('repository unavailable')).mockReturnValueOnce(request.promise);
        const save = jest.fn();
        const parameters = (application: models.Application) => (
            <Context.Provider value={ctxValue}>
                <ApplicationParameters application={application} save={save} pageNumber={0} setPageNumber={jest.fn()} />
            </Context.Provider>
        );
        const user = userEvent.setup();
        const {container, rerender} = render(parameters(app));
        expect(await screen.findByRole('alert')).toHaveTextContent('repository unavailable');
        await user.click(screen.getAllByRole('button', {name: 'Edit'})[multiSource ? 1 : 0]);
        const editor = container.querySelector('textarea');
        await user.clear(editor);
        await user.type(editor, 'replicaCount: 7\n');
        await user.click(screen.getByRole('button', {name: 'Retry'}));

        expect(appDetails).toHaveBeenNthCalledWith(2, source, 'test-app', 'default', 0, 0);
        expect(appDetails.mock.calls[1][0].helm).toEqual({valuesObject: {replicaCount: 1}});
        expect(appDetails.mock.calls[1][0].helm).not.toHaveProperty('values');
        expect(container.querySelector('textarea')).toBe(editor);
        expect(editor).toHaveValue('replicaCount: 7\n');
        expect(screen.getByRole('alert')).toHaveTextContent('repository unavailable');
        expect(screen.getByRole('button', {name: 'Retry'})).toBeDisabled();
        rerender(parameters({...app, metadata: {...app.metadata, resourceVersion: '2'}}));
        expect(appDetails).toHaveBeenCalledTimes(2);
        expect(container.querySelector('textarea')).toBe(editor);
        await act(async () => {
            if (retrySuccess) {
                request.resolve({type: 'Helm', helm: {parameters: [], valueFiles: [], fileParameters: []}} as models.RepoAppDetails);
            } else {
                request.reject(new Error('retry failed'));
            }
        });

        expect(container.querySelector('textarea')).toBe(editor);
        expect(editor).toHaveValue('replicaCount: 7\n');
        expect(save).not.toHaveBeenCalled();
        if (retrySuccess) {
            expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        } else {
            expect(screen.getByRole('alert')).toHaveTextContent('retry failed');
            expect(screen.getByRole('button', {name: 'Retry'})).toBeEnabled();

            const staleRetry = deferredRepoDetails();
            appDetails.mockReturnValueOnce(staleRetry.promise).mockResolvedValueOnce({type: 'Helm', helm: {parameters: [{name: 'current-context-default', value: 'ready'}]}});
            await user.click(screen.getByRole('button', {name: 'Retry'}));
            rerender(parameters({...app, spec: {...app.spec, project: 'other-project'}}));
            expect(await screen.findByText('current-context-default')).toBeInTheDocument();
            await act(async () => staleRetry.reject(new Error('obsolete retry failed')));
            expect(screen.queryByRole('alert')).not.toBeInTheDocument();
            expect(screen.getByText('current-context-default')).toBeInTheDocument();
        }
    });

    test.each(['Save', 'Cancel'])('defers incompatible Plugin announcements until %s', async action => {
        const app = multiSourceApp();
        const source = app.spec.sources[0];
        source.plugin.parameters = [{name: 'options', string: 'configured'}];
        if (!multiSource) {
            app.spec.source = source;
            delete app.spec.sources;
        }
        const request = deferredRepoDetails();
        appDetails.mockRejectedValueOnce(new Error('repository unavailable')).mockReturnValueOnce(request.promise);
        const save = jest.fn().mockResolvedValue(undefined);
        const user = userEvent.setup();
        renderWithContext(<ApplicationParameters application={app} save={save} pageNumber={0} setPageNumber={jest.fn()} />);

        expect(await screen.findByRole('alert')).toHaveTextContent('repository unavailable');
        await user.click(screen.getAllByRole('button', {name: 'Edit'})[multiSource ? 1 : 0]);
        const editor = screen.getByDisplayValue('configured');
        await user.clear(editor);
        await user.type(editor, 'user-draft');
        await user.click(screen.getByRole('button', {name: 'Retry'}));
        await act(async () =>
            request.resolve({
                type: 'Plugin',
                plugin: {
                    parametersAnnouncement: [
                        {name: 'options', collectionType: 'map'},
                        {name: 'fresh-default', string: 'ready'}
                    ]
                }
            } as models.RepoAppDetails)
        );

        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        expect(screen.getByDisplayValue('user-draft')).toBe(editor);
        expect(screen.queryByText('fresh-default')).not.toBeInTheDocument();
        const pluginPanel = screen.getByText('PLUGIN').parentElement.parentElement;
        await user.click(within(pluginPanel).getByRole('button', {name: action, exact: true}));
        expect(await screen.findByText('fresh-default')).toBeInTheDocument();
        if (action === 'Save') {
            await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
            const saved = save.mock.calls[0][0] as models.Application;
            const savedSource = multiSource ? saved.spec.sources[0] : saved.spec.source;
            expect(savedSource.plugin.parameters).toEqual([{name: 'options', string: 'user-draft'}]);
            expect(savedSource.plugin.parameters[0]).not.toHaveProperty('map');
        } else {
            expect(save).not.toHaveBeenCalled();
        }
    });

    test.each([
        {olderFirst: false, currentFailure: false},
        {olderFirst: true, currentFailure: false},
        {olderFirst: false, currentFailure: true},
        {olderFirst: true, currentFailure: true}
    ])('ignores a superseded result (older first: $olderFirst, current failure: $currentFailure)', async ({olderFirst, currentFailure}) => {
        const older = deferredRepoDetails();
        const current = deferredRepoDetails();
        appDetails.mockImplementation((source: models.ApplicationSource) => (source.targetRevision === 'main' ? older.promise : current.promise));
        const appSource = {repoURL: 'https://charts.example.com', chart: 'web-app', targetRevision: 'main'} as models.ApplicationSource;
        const app = multiSourceApp();
        if (multiSource) {
            app.spec.sources = [appSource];
        } else {
            app.spec.source = appSource;
            delete app.spec.sources;
        }
        const parameters = (application: models.Application) => (
            <Context.Provider value={ctxValue}>
                <ApplicationParameters application={application} pageNumber={0} setPageNumber={jest.fn()} />
            </Context.Provider>
        );
        const {rerender} = render(parameters(app));
        await waitFor(() => expect(appDetails).toHaveBeenCalledTimes(1));
        const updatedSource = {...appSource, targetRevision: 'release'};
        rerender(parameters({...app, spec: {...app.spec, ...(multiSource ? {sources: [updatedSource]} : {source: updatedSource})}}));
        await waitFor(() => expect(appDetails).toHaveBeenLastCalledWith(updatedSource, 'test-app', 'default', 0, 0));

        const finishOlder = () =>
            act(async () => {
                if (currentFailure) {
                    older.resolve({type: 'Helm', helm: {parameters: [{name: 'old-default', value: 'stale'}]}} as models.RepoAppDetails);
                } else {
                    older.reject(new Error('obsolete lookup failed'));
                }
            });
        const finishCurrent = () =>
            act(async () => {
                if (currentFailure) {
                    current.reject(new Error('current lookup failed'));
                } else {
                    current.resolve({type: 'Helm', helm: {parameters: [{name: 'current-default', value: 'current'}]}} as models.RepoAppDetails);
                }
            });

        if (olderFirst) {
            await finishOlder();
            expect(screen.queryByRole('alert')).not.toBeInTheDocument();
            expect(screen.queryByText('old-default')).not.toBeInTheDocument();
            await finishCurrent();
        } else {
            await finishCurrent();
            await finishOlder();
        }

        if (currentFailure) {
            expect(screen.getByRole('alert')).toHaveTextContent('current lookup failed');
        } else {
            expect(screen.queryByRole('alert')).not.toBeInTheDocument();
            expect(screen.getByText('current-default')).toBeInTheDocument();
        }
        expect(screen.queryByText('obsolete lookup failed')).not.toBeInTheDocument();
        expect(screen.queryByText('old-default')).not.toBeInTheDocument();
    });
});

test('does not restart single-source lookups for watch updates', async () => {
    const app = multiSourceApp();
    app.spec.source = app.spec.sources[0];
    delete app.spec.sources;
    const request = deferredRepoDetails();
    const appDetails = services.repos.appDetails as jest.Mock;
    appDetails.mockReset().mockReturnValue(request.promise);
    const parameters = (version: string) => (
        <Context.Provider value={ctxValue}>
            <ApplicationParameters application={{...app, metadata: {...app.metadata, resourceVersion: version}, status: {reconciledAt: version} as any}} />
        </Context.Provider>
    );
    const {rerender} = render(parameters('1'));
    await waitFor(() => expect(appDetails).toHaveBeenCalledTimes(1));
    for (const version of ['2', '3', '4']) {
        rerender(parameters(version));
        expect(appDetails).toHaveBeenCalledTimes(1);
    }
    await act(async () => request.resolve({type: 'Plugin', plugin: {}} as models.RepoAppDetails));
    const name = await screen.findByDisplayValue('my-plugin');
    rerender(parameters('5'));
    expect(screen.getByDisplayValue('my-plugin')).toBe(name);
    expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    expect(appDetails).toHaveBeenCalledTimes(1);
});

test.each([false, true])('hides stored single-source details while another source loads (previous failure: %s)', async previousFailure => {
    const app = multiSourceApp();
    app.spec.source = {repoURL: 'https://charts.example.com', chart: 'web-app', targetRevision: 'main'} as models.ApplicationSource;
    delete app.spec.sources;
    const request = deferredRepoDetails();
    const appDetails = services.repos.appDetails as jest.Mock;
    appDetails.mockReset();
    if (previousFailure) {
        appDetails.mockRejectedValueOnce(new Error('previous repository unavailable'));
    } else {
        appDetails.mockResolvedValueOnce({type: 'Helm', helm: {parameters: []}});
    }
    appDetails.mockReturnValueOnce(request.promise);
    const parameters = (application: models.Application) => (
        <Context.Provider value={ctxValue}>
            <ApplicationParameters application={application} />
        </Context.Provider>
    );
    const {rerender} = render(parameters(app));
    expect(await screen.findByText('HELM')).toBeInTheDocument();
    const updatedSource = {
        repoURL: 'https://git.example.com/plugin.git',
        path: 'manifests',
        targetRevision: 'release',
        plugin: {name: 'current-plugin'}
    } as models.ApplicationSource;
    rerender(parameters({...app, spec: {...app.spec, source: updatedSource}}));
    expect(screen.queryByText('HELM')).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByText('Loading...')).toBeInTheDocument();
    await act(async () => request.resolve({type: 'Plugin', plugin: {}} as models.RepoAppDetails));
    expect(await screen.findByDisplayValue('current-plugin')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});

test.each(['project', 'name'])('reloads multi-source details when only the application %s changes', async field => {
    const app = multiSourceApp();
    const appDetails = services.repos.appDetails as jest.Mock;
    appDetails
        .mockReset()
        .mockRejectedValueOnce(new Error('previous context unavailable'))
        .mockResolvedValue({type: 'Plugin', plugin: {parametersAnnouncement: [{name: 'current-context-default', string: 'ready'}]}});
    const parameters = (application: models.Application) => (
        <Context.Provider value={ctxValue}>
            <ApplicationParameters application={application} pageNumber={0} setPageNumber={jest.fn()} />
        </Context.Provider>
    );
    const {rerender} = render(parameters(app));
    expect(await screen.findByRole('alert')).toHaveTextContent('previous context unavailable');
    const updatedApp = field === 'project' ? {...app, spec: {...app.spec, project: 'other-project'}} : {...app, metadata: {...app.metadata, name: 'other-app'}};
    rerender(parameters(updatedApp));
    await waitFor(() => expect(appDetails).toHaveBeenLastCalledWith(app.spec.sources[0], updatedApp.metadata.name, updatedApp.spec.project, 0, 0));
    expect(await screen.findByText('current-context-default')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});

test('a failed source does not hide a successful source or its collapsed summary', async () => {
    const app = multiSourceApp();
    app.spec.sources[0].path = 'manifests';
    app.spec.sources.push({repoURL: 'https://charts.example.com', chart: 'web-app', targetRevision: '1.0.0'} as models.ApplicationSource);
    (services.repos.appDetails as jest.Mock).mockImplementation((source: models.ApplicationSource) =>
        source.plugin ? Promise.reject(new Error('plugin repository unavailable')) : Promise.resolve({type: 'Helm', helm: {valueFiles: [], parameters: [], fileParameters: []}})
    );
    const Panels = () => {
        const [collapsedSources, setCollapsedSources] = React.useState<boolean[]>([]);
        return (
            <ApplicationParameters
                application={app}
                pageNumber={0}
                setPageNumber={jest.fn()}
                collapsedSources={collapsedSources}
                handleCollapse={(index, collapsed) => setCollapsedSources(previous => Object.assign([...previous], {[index]: collapsed}))}
            />
        );
    };
    const user = userEvent.setup();
    const {container} = renderWithContext(<Panels />);

    expect(await screen.findByRole('alert')).toHaveTextContent('plugin repository unavailable');
    expect(await screen.findByText('HELM')).toBeInTheDocument();
    const helmPanel = screen.getByText('HELM').closest('.white-box');
    expect(screen.getByText('web-app:1.0.0')).toBeInTheDocument();
    expect(screen.getAllByRole('alert')).toHaveLength(1);
    await user.click(helmPanel.querySelector('.fa-angle-up'));
    expect(await screen.findByText('CHART=web-app, REVISION=1.0.0')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('plugin repository unavailable');
    expect(screen.getByDisplayValue('my-plugin')).toBeInTheDocument();
    expect(screen.queryByText('HELM')).not.toBeInTheDocument();
    await user.click(container.querySelector('.fa-angle-up'));
    expect(await screen.findByText('PATH=manifests, REVISION=main, ENV=[FOO=bar, ENVIRONMENT=prod]')).toBeInTheDocument();
    expect(screen.getByText('CHART=web-app, REVISION=1.0.0')).toBeInTheDocument();
});
