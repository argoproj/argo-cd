import {render, screen} from '@testing-library/react';
import * as React from 'react';

import * as models from '../../../shared/models';
import {Context} from '../../../shared/context';
import {ApplicationParameters} from './application-parameters';

// lodash-es ships as untransformed ESM under node_modules and is not covered by the jest
// transform config, so map the single helper this module uses to a CJS-friendly stub.
jest.mock('lodash-es', () => ({
    __esModule: true,
    cloneDeep: (value: any) => (value == null ? value : JSON.parse(JSON.stringify(value)))
}));

// The expanded per-source panel resolves its repo details through argo-ui's DataLoader.
// Replace it with a synchronous passthrough that resolves `load(input)` and renders children,
// so the expanded plugin section renders deterministically without a backend.
jest.mock('argo-ui', () => {
    const actual = jest.requireActual('argo-ui');
    const react = require('react');
    const MockDataLoader = ({load, input, children}: any) => {
        const [state, setState] = react.useState<{ready: boolean; data: any}>({ready: false, data: undefined});
        react.useEffect(() => {
            let alive = true;
            Promise.resolve(load(input)).then((data: any) => alive && setState({ready: true, data}));
            return () => {
                alive = false;
            };
        }, []);
        return state.ready ? children(state.data) : null;
    };
    return {...actual, DataLoader: MockDataLoader};
});

// The expanded panel loads per-source repo details via services.repos.appDetails; return a
// Plugin-type detail so gatherDetails takes the Plugin branch.
jest.mock('../../../shared/services', () => ({
    services: {
        repos: {
            appDetails: () => Promise.resolve({type: 'Plugin', plugin: {}})
        }
    }
}));

// Paginate pulls in a view-preferences DataLoader that is irrelevant to what we assert here,
// so replace it with a passthrough that simply renders its children for the provided data.
jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Paginate: ({data, children}: {data: any[]; children: (d: any[]) => React.ReactNode}) => <>{children(data)}</>
}));

// The add-source panel is not exercised by these tests and depends on backend services.
jest.mock('./source-panel', () => ({SourcePanel: () => null}));

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

test('collapsed multi-source summary shows plugin env values', () => {
    renderWithContext(<ApplicationParameters application={multiSourceApp()} collapsedSources={[true]} handleCollapse={() => undefined} />);

    expect(screen.getByText(/ENV=\[FOO=bar, ENVIRONMENT=prod\]/)).toBeInTheDocument();
});

test('expanded multi-source plugin panel renders NAME and ENV from the per-source plugin', async () => {
    // collapsedSources[0] = false -> the expanded panel renders and reads spec.sources[0].plugin.
    // Before the fix it read the (absent) single-source spec.source.plugin, so NAME/ENV were blank.
    renderWithContext(<ApplicationParameters application={multiSourceApp()} collapsedSources={[false]} handleCollapse={() => undefined} />);

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
});
