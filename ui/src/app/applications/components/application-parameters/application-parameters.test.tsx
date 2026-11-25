import {render, screen, waitFor, within} from '@testing-library/react';
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

jest.mock('./application-parameters-source', () => {
    const actual = jest.requireActual('./application-parameters-source');
    return {
        ...actual,
        ApplicationParametersSource: jest.fn((props: any) => actual.ApplicationParametersSource(props))
    };
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

describe('validateHelmValues', () => {
    const {validateHelmValues} = require('./application-parameters');
    test('accepts valid YAML map', () => {
        expect(validateHelmValues('image:\n  tag: stable')).toBeNull();
    });

    test('rejects malformed YAML', () => {
        expect(validateHelmValues('image:ef\n  tag: stable')).toBe('Values must be valid YAML');
    });

    test('rejects non-map YAML (array)', () => {
        expect(validateHelmValues('- item')).toBe('Values must be a map');
    });

    test('rejects non-map YAML (scalar)', () => {
        expect(validateHelmValues('2026-09-04')).toBe('Values must be a map');
    });

    test('rejects non-map YAML (null)', () => {
        expect(validateHelmValues('null')).toBe('Values must be a map');
    });

    test('accepts empty string gracefully', () => {
        expect(validateHelmValues('')).toBeNull();
    });
});

test('multi-source helm values do not clear invalid yaml on save', async () => {
    const {ApplicationParametersSource} = require('./application-parameters-source');
    const mockSave = jest.fn();
    
    // Create an app that simulates the state just before saving.
    // The user has typed invalid YAML in the UI, which updates the local `app` state's helm.values.
    const app = multiSourceApp();
    app.spec.sources[0].plugin = undefined;
    app.spec.sources[0].helm = {
        values: 'invalid: yaml: :\n',
        valuesObject: {valid: 'yaml'}
    };
    app.spec.sources[0].chart = 'my-chart'; // to make it helm
    
    // Mock the DataLoader to resolve the source as Helm
    jest.spyOn(require('../../../shared/services').services.repos, 'appDetails')
        .mockResolvedValueOnce({type: 'Helm', helm: {}});
        
    renderWithContext(<ApplicationParameters application={app} save={mockSave} collapsedSources={[false]} handleCollapse={() => undefined} />);
    
    // Wait for the ApplicationParametersSource component to be rendered (it renders after DataLoader resolves)
    const {waitFor} = require('@testing-library/react');
    await waitFor(() => expect(ApplicationParametersSource).toHaveBeenCalled());
    
    // The ApplicationParameters component passes saveBottom to ApplicationParametersSource
    const calls = (ApplicationParametersSource as jest.Mock).mock.calls;
    const saveBottom = calls[calls.length - 1][0].saveBottom;
    
    // The user clicks save, passing the current form state to saveBottom
    const formInputApp = JSON.parse(JSON.stringify(app));
    await saveBottom(formInputApp);
    
    // We expect the original save prop to have been called
    expect(mockSave).toHaveBeenCalled();
    
    // Crucially, the invalid YAML string should NOT be cleared from the payload.
    const savedApp = mockSave.mock.calls[0][0];
    expect(savedApp.spec.sources[0].helm.values).toBe('invalid: yaml: :\n');
});

test('single-source helm values editor handles invalid yaml gracefully and prevents autosave', async () => {
    const { fireEvent, screen } = require('@testing-library/react');
    const mockSave = jest.fn();

    // Create a single-source app with valid Helm values initially
    const app = {
        metadata: {name: 'test-app'},
        spec: {
            project: 'default',
            source: {
                repoURL: 'https://example.com/repo',
                path: 'my-chart',
                helm: {
                    values: 'valid: true\n'
                }
            }
        }
    } as any;

    // Mock DataLoader resolution for single source to render Helm panel
    jest.spyOn(require('../../../shared/services').services.repos, 'appDetails')
        .mockResolvedValueOnce({type: 'Helm', helm: {}});

    // Render in noReadonlyMode=true to enable auto-save on input change
    renderWithContext(<ApplicationParameters application={app} save={mockSave} noReadonlyMode={true} />);

    // Find the actual Helm values editor textarea
    const { waitFor } = require('@testing-library/react');
    const textarea = await waitFor(() => {
        const ta = document.querySelector('textarea');
        if (!ta) throw new Error('Textarea not found');
        return ta;
    });

    // Simulate the user typing malformed YAML exactly once
    fireEvent.change(textarea, { target: { value: 'invalid: yaml: :\n' } });

    // Wait for validation to report the invalid YAML error in the UI
    expect(await screen.findByText('Values must be valid YAML')).toBeInTheDocument();

    // The formDidUpdate callback attempts autosave. Verify it did NOT save the invalid YAML.
    const invalidCalls = mockSave.mock.calls.filter(call => 
        call[0].spec.source?.helm?.values === 'invalid: yaml: :\n'
    );
    expect(invalidCalls).toHaveLength(0);

    // The UI should remain rendered without crashing
    expect(textarea).toBeInTheDocument();
})
test('multi-source default (no collapse state) renders the expanded panel, not the raw-text summary', async () => {
    // The details page initializes collapsedSources as an empty array, so every entry is undefined (see argoproj/argo-cd#30942).
    (services.repos.appDetails as jest.Mock).mockResolvedValue({type: 'Plugin', plugin: {}});
    renderWithContext(<ApplicationParameters application={multiSourceApp()} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[]} handleCollapse={() => undefined} />);

    expect(await screen.findByDisplayValue('my-plugin')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('FOO')).toBeInTheDocument();
    expect(screen.queryByText(/ENV=\[FOO=bar, ENVIRONMENT=prod\]/)).not.toBeInTheDocument();
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
