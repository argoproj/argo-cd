import {act, render, waitFor} from '@testing-library/react';
import * as React from 'react';

import {FormApi} from 'argo-ui';
import * as models from '../../../shared/models';
import {ApplicationCreatePanel} from './application-create-panel';

// lodash-es ships as untransformed ESM under node_modules and is not covered by the jest
// transform config, so map the helpers this module uses to CJS-friendly stubs.
jest.mock('lodash-es', () => ({
    __esModule: true,
    cloneDeep: (value: any) => (value == null ? value : JSON.parse(JSON.stringify(value))),
    debounce: (fn: (...args: any[]) => any, wait: number) => {
        let timer: ReturnType<typeof setTimeout>;
        const debounced = (...args: any[]) => {
            clearTimeout(timer);
            timer = setTimeout(() => fn(...args), wait);
        };
        debounced.cancel = () => clearTimeout(timer);
        return debounced;
    }
}));

// Replace argo-ui's DataLoader with a passthrough that resolves `load(input)` and renders children.
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

jest.mock('../../../shared/services', () => ({
    services: {
        authService: {settings: () => Promise.resolve({})},
        projects: {list: () => Promise.resolve([])},
        clusters: {list: () => Promise.resolve([])},
        repos: {list: () => Promise.resolve([])}
    }
}));

// The source and parameters sections depend on backend services and are not exercised here.
jest.mock('./source-panel', () => ({SourcePanel: () => null}));
jest.mock('../application-parameters/application-parameters', () => ({ApplicationParameters: () => null}));
jest.mock('../../../shared/components/yaml-editor/yaml-editor', () => ({YamlEditor: () => null}));

const renderPanel = (onAppChanged: (app: models.Application) => any, onFormApi: (api: FormApi) => void) => (
    <ApplicationCreatePanel app={{} as models.Application} onAppChanged={onAppChanged} createApp={jest.fn()} getFormApi={onFormApi} />
);

describe('ApplicationCreatePanel', () => {
    test('reports the latest form values after the parent re-renders with a new onAppChanged', async () => {
        let formApi: FormApi;
        const first = jest.fn();
        const {rerender} = render(renderPanel(first, api => (formApi = api)));
        await waitFor(() => expect(formApi).toBeDefined());

        act(() => {
            formApi.setValue('metadata.name', 'my-app');
        });

        // The parent creates a new onAppChanged on every render, e.g. when the application list updates.
        const second = jest.fn();
        rerender(renderPanel(second, api => (formApi = api)));

        await waitFor(() => expect(second).toHaveBeenCalled(), {timeout: 2000});
        expect(second).toHaveBeenLastCalledWith(expect.objectContaining({metadata: expect.objectContaining({name: 'my-app'})}));
        expect(first).not.toHaveBeenCalled();
    });

    test('drops the pending update when the panel unmounts', async () => {
        let formApi: FormApi;
        const onAppChanged = jest.fn();
        const {unmount} = render(renderPanel(onAppChanged, api => (formApi = api)));
        await waitFor(() => expect(formApi).toBeDefined());

        act(() => {
            formApi.setValue('metadata.name', 'my-app');
        });
        unmount();

        await new Promise(resolve => setTimeout(resolve, 1000));
        expect(onAppChanged).not.toHaveBeenCalled();
    });
});
