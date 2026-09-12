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

    expect(screen.getByText(/ENV=FOO=bar ENVIRONMENT=prod/)).toBeInTheDocument();
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
