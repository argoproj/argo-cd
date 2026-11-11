import * as React from 'react';
import {act, render, waitFor} from '@testing-library/react';
import {FormApi} from 'argo-ui';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {SourcePanel} from './source-panel';

// lodash-es ships as untransformed ESM under node_modules and is not covered by the jest
// transform config, so map the single helper this module uses to a CJS-friendly stub.
jest.mock('lodash-es', () => ({
    __esModule: true,
    cloneDeep: (value: any) => (value == null ? value : JSON.parse(JSON.stringify(value)))
}));

jest.mock('argo-ui', () => {
    const actual = jest.requireActual('argo-ui');
    const react = require('react');
    // Loads on mount and whenever the input changes, like DataLoader, without needing the app context.
    const MockDataLoader = ({load, input, children}: any) => {
        const [state, setState] = react.useState({ready: false, data: undefined as any});
        const key = JSON.stringify(input);
        react.useEffect(() => {
            let alive = true;
            Promise.resolve(load(input)).then((data: any) => alive && setState({ready: true, data}));
            return () => {
                alive = false;
            };
        }, [key]);
        return state.ready ? children(state.data) : null;
    };
    return {...actual, DataLoader: MockDataLoader};
});

jest.mock('../../../shared/services', () => ({
    services: {
        repos: {
            list: jest.fn(),
            revisions: jest.fn(),
            ociTags: jest.fn(),
            apps: jest.fn(),
            charts: jest.fn(),
            appDetails: jest.fn()
        }
    }
}));

const repoURL = 'https://git.example.com/team-a/config.git';

describe('SourcePanel', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        jest.mocked(services.repos.list).mockResolvedValue([{repo: repoURL, type: 'git'} as models.Repository]);
        jest.mocked(services.repos.revisions).mockResolvedValue({branches: ['main'], tags: []} as any);
        jest.mocked(services.repos.apps).mockResolvedValue([]);
        jest.mocked(services.repos.appDetails).mockResolvedValue({type: 'Directory'} as any);
    });

    it("loads revisions of a new source with the application's project", async () => {
        let formApi: FormApi;
        const app = {metadata: {name: 'guestbook'}, spec: {project: 'team-a', sources: []}} as unknown as models.Application;
        // The parameters section's EditablePanel reads notifications off the app Context.
        render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <SourcePanel appCurrent={app} onSubmitFailure={jest.fn()} updateApp={jest.fn()} getFormApi={api => (formApi = api)} />
            </Context.Provider>
        );

        await waitFor(() => expect(formApi).toBeDefined());
        act(() => formApi.setValue('spec.source.repoURL', repoURL));

        await waitFor(() => expect(services.repos.revisions).toHaveBeenCalledWith(repoURL, 'team-a'));
        expect(services.repos.revisions).not.toHaveBeenCalledWith(repoURL, '');
    });
});
