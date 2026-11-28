import {render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as React from 'react';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {AuthSettingsCtx, Context} from '../../../shared/context';
import {ApplicationParameters} from './application-parameters';

jest.mock('../../../shared/services', () => ({
    services: {
        repos: {
            appDetails: jest.fn()
        },
        authService: {
            plugins: jest.fn().mockResolvedValue([]),
            settings: jest.fn().mockResolvedValue({kustomizeVersions: []})
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

jest.mock('../revision-form-field/revision-form-field', () => ({
    RevisionFormField: () => null
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

test.each([
    [true, 'apps/test-app'],
    [false, 'test-app']
])('requests repo details with the app name qualified per apps-in-any-namespace (enabled: %s)', async (appsInAnyNamespaceEnabled, expectedName) => {
    const appDetails = services.repos.appDetails as jest.Mock;
    appDetails.mockReset();
    appDetails.mockResolvedValue({type: 'Plugin', plugin: {}});
    const app = multiSourceApp();
    app.metadata.namespace = 'apps';
    renderWithContext(
        <AuthSettingsCtx.Provider value={{appsInAnyNamespaceEnabled} as models.AuthSettings}>
            <ApplicationParameters application={app} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[false]} handleCollapse={() => undefined} />
        </AuthSettingsCtx.Provider>
    );

    expect(await screen.findByDisplayValue('my-plugin')).toBeInTheDocument();
    expect(appDetails).toHaveBeenCalledWith(expect.objectContaining({repoURL: 'https://github.com/example/repo'}), expectedName, 'default', 0, 0);
});

describe.each([false, true])('ApplicationParameters repository details failure (multi-source: %s)', multiSource => {
    const appDetails = services.repos.appDetails as jest.Mock;
    const warning = 'Unable to load repository details. The source settings shown may be incomplete: repository unavailable';

    const renderSource = (source: Partial<models.ApplicationSource>) => {
        const appSource = {repoURL: 'https://github.com/example/repo', targetRevision: 'main', ...source} as models.ApplicationSource;
        const app = multiSourceApp();
        if (multiSource) {
            app.spec.sources = [appSource];
        } else {
            app.spec.source = appSource;
            delete app.spec.sources;
        }
        renderWithContext(<ApplicationParameters application={app} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[]} handleCollapse={jest.fn()} />);
    };

    beforeEach(() => {
        appDetails.mockReset();
    });

    test('warns and keeps configured Helm settings', async () => {
        appDetails.mockRejectedValue(new Error('repository unavailable'));
        renderSource({chart: 'web-app', helm: {valueFiles: ['production.yaml'], parameters: [{name: 'replicas', value: '3'}]}});

        expect(await screen.findByRole('alert')).toHaveTextContent(warning);
        expect(screen.getByText('HELM')).toBeInTheDocument();
        expect(screen.queryByText('DIRECTORY')).not.toBeInTheDocument();
        expect(screen.getByText('production.yaml')).toBeInTheDocument();
        expect(screen.getByText('replicas')).toBeInTheDocument();
    });

    (multiSource ? test : test.skip)('places the multi-source warning after the source title', async () => {
        appDetails.mockRejectedValue(new Error('repository unavailable'));
        renderSource({chart: 'web-app'});

        const alert = await screen.findByRole('alert');
        const title = screen.getByText(/^Source 1: TYPE=Helm/);
        expect(title.compareDocumentPosition(alert) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });

    test('warns and keeps configured Plugin settings', async () => {
        appDetails.mockRejectedValue(new Error('repository unavailable'));
        renderSource({path: 'manifests', plugin: {name: 'my-plugin', env: [{name: 'FOO', value: 'bar'}]}});

        expect(await screen.findByRole('alert')).toHaveTextContent(warning);
        expect(screen.getByText('PLUGIN')).toBeInTheDocument();
        expect(screen.queryByText('DIRECTORY')).not.toBeInTheDocument();
        expect(screen.getByDisplayValue('my-plugin')).toBeInTheDocument();
        expect(screen.getByDisplayValue('FOO')).toBeInTheDocument();
    });

    test('shows no warning when the request succeeds', async () => {
        appDetails.mockResolvedValue({type: 'Helm', helm: {valueFiles: [], parameters: [], fileParameters: []}});
        renderSource({chart: 'web-app'});

        expect(await screen.findByText('HELM')).toBeInTheDocument();
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
});

describe.each([false, true])('ApplicationParameters Kustomize image save (multi-source: %s)', multiSource => {
    const appDetails = services.repos.appDetails as jest.Mock;

    const saveWithEdit = async (images: string[], edit: (row: (name: string) => HTMLElement) => Promise<void>) => {
        const source = {repoURL: 'https://github.com/example/repo', path: 'manifests', targetRevision: 'main', kustomize: {namespace: 'old', images}} as models.ApplicationSource;
        const app = multiSourceApp();
        if (multiSource) {
            app.spec.sources = [source];
        } else {
            app.spec.source = source;
            delete app.spec.sources;
        }
        appDetails.mockResolvedValue({type: 'Kustomize', kustomize: {images: ['mysql', 'nginx']}});
        const save = jest.fn().mockResolvedValue(undefined);
        const user = userEvent.setup();
        renderWithContext(<ApplicationParameters application={app} save={save} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[]} handleCollapse={jest.fn()} />);
        expect(await screen.findByText('KUSTOMIZE')).toBeInTheDocument();
        const panel = screen.getByText('KUSTOMIZE').parentElement.parentElement;
        await user.click(within(panel).getByRole('button', {name: 'Edit', exact: true}));
        await edit(name => screen.getByText(name).closest('.white-box__details-row') as HTMLElement);
        await user.click(within(panel).getByRole('button', {name: 'Save', exact: true}));
        await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
        const saved = save.mock.calls[0][0] as models.Application;
        return (multiSource ? saved.spec.sources[0] : saved.spec.source).kustomize.images;
    };

    test('keeps rename-only and digest overrides when another field is saved', async () => {
        const images = ['mysql=mariadb-next', 'nginx@sha256:0123'];
        const user = userEvent.setup();
        const saved = await saveWithEdit(images, async () => {
            const namespace = screen.getByDisplayValue('old');
            await user.clear(namespace);
            await user.type(namespace, 'new');
        });

        expect(saved).toEqual(images);
    });

    test('still drops an override whose tag was cleared', async () => {
        const user = userEvent.setup();
        const saved = await saveWithEdit(['mysql=mariadb-next', 'nginx:1.2'], async row => {
            await user.clear(within(row('nginx')).getAllByRole('textbox')[1]);
        });

        expect(saved).toEqual(['mysql=mariadb-next']);
    });
});
