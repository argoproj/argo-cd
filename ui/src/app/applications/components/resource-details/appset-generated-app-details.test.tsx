import {render, screen, waitFor} from '@testing-library/react';
import * as React from 'react';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {AppSetGeneratedAppDetails} from './appset-generated-app-details';

const mockGet = jest.fn();

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            get: (...args: any[]) => mockGet(...args)
        }
    }
}));

jest.mock('../application-node-info/application-node-info', () => ({
    ApplicationNodeInfo: (props: any) => <div data-testid='node-info'>{JSON.stringify({kind: props.live?.kind, hasStatus: props.live?.status != null})}</div>
}));

jest.mock('../application-resource-events/application-resource-events', () => ({
    ApplicationResourceEvents: (props: any) => <div data-testid='app-events'>{`${props.applicationName}/${props.applicationNamespace}`}</div>
}));

jest.mock('../utils', () => ({
    ComparisonStatusIcon: () => null,
    HealthStatusIcon: () => null,
    nodeKey: (node: any) => [node.group, node.kind, node.namespace, node.name].join('/'),
    getAppUrl: (app: any) => `applications/${app.metadata.namespace}/${app.metadata.name}`
}));

const guestbook = {
    apiVersion: 'argoproj.io/v1alpha1',
    kind: 'Application',
    metadata: {name: 'guestbook', namespace: 'argocd'},
    spec: {project: 'default', source: {repoURL: 'https://github.com/argoproj/argocd-example-apps.git'}},
    status: {sync: {status: 'Synced'}, health: {status: 'Healthy'}}
} as models.Application;

const node = {
    group: 'argoproj.io',
    version: 'v1alpha1',
    kind: 'Application',
    namespace: 'argocd',
    name: 'guestbook',
    resourceVersion: '123',
    parentRefs: [],
    info: [],
    status: 'Synced',
    health: {status: 'Healthy'}
} as models.ResourceNode & {status: string; health: models.HealthStatus};

let goto: jest.Mock;

const renderComponent = (search = '', appNode: models.ResourceNode = node) => {
    goto = jest.fn();
    return render(
        <Context.Provider
            value={
                {
                    history: {location: {search}},
                    navigation: {goto},
                    notifications: {show: jest.fn()},
                    popup: {},
                    baseHref: '/'
                } as any
            }>
            <AppSetGeneratedAppDetails node={appNode} />
        </Context.Provider>
    );
};

describe('AppSetGeneratedAppDetails', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockGet.mockResolvedValue(guestbook);
    });

    it('loads the generated Application itself and shows its full manifest inline', async () => {
        renderComponent();

        await waitFor(() => expect(mockGet).toHaveBeenCalledWith('guestbook', 'argocd', 'application'));

        // The selected Application's own name is rendered in the panel header instead of navigating away.
        expect(await screen.findByText('guestbook')).toBeInTheDocument();

        // The live manifest passed to the node-info panel is the complete Application object, status included.
        const nodeInfo = await screen.findByTestId('node-info');
        expect(JSON.parse(nodeInfo.textContent)).toEqual({kind: 'Application', hasStatus: true});
    });

    it('reloads the manifest when the watched node reports a new resource version', async () => {
        const value = {history: {location: {search: ''}}, navigation: {goto: jest.fn()}, notifications: {show: jest.fn()}, popup: {}, baseHref: '/'} as any;
        const {rerender} = render(
            <Context.Provider value={value}>
                <AppSetGeneratedAppDetails node={node} />
            </Context.Provider>
        );
        await waitFor(() => expect(mockGet).toHaveBeenCalledTimes(1));

        rerender(
            <Context.Provider value={value}>
                <AppSetGeneratedAppDetails node={{...node, resourceVersion: '999'}} />
            </Context.Provider>
        );

        await waitFor(() => expect(mockGet).toHaveBeenCalledTimes(2));
    });

    it('shows the application events for the selected Application on the EVENTS tab', async () => {
        renderComponent('?tab=events');

        const events = await screen.findByTestId('app-events');
        expect(events).toHaveTextContent('guestbook/argocd');
    });

    it('offers a DETAILS action that opens the Application page', async () => {
        renderComponent();

        const detailsButton = ((await screen.findByText('DETAILS')).closest('button')) as HTMLElement;
        detailsButton.click();

        expect(goto).toHaveBeenCalledWith('/applications/argocd/guestbook');
    });
});
