import {KeybindingProvider} from 'argo-ui/v2';
import * as React from 'react';
import * as renderer from 'react-test-renderer';

import {Context} from '../../../shared/context';
import {ApplicationsDetailsAppDropdown} from './application-details-app-dropdown';

// react-test-renderer cannot host real DOM nodes; collapse the portal so the
// panel is rendered inline as part of the test tree.
jest.mock('react-dom', () => {
    const actual = jest.requireActual('react-dom');
    return {
        ...actual,
        createPortal: (node: React.ReactNode) => node
    };
});

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            list: jest.fn(() =>
                Promise.resolve({
                    items: [
                        {metadata: {name: 'app-one', namespace: 'argocd'}, kind: 'Application'},
                        {metadata: {name: 'app-two', namespace: 'argocd'}, kind: 'Application'}
                    ]
                })
            )
        },
        viewPreferences: {
            getPreferences: jest.fn(() => ({
                subscribe: (cb: (p: {theme: string}) => void) => {
                    cb({theme: 'light'});
                    return {unsubscribe: () => undefined};
                }
            }))
        }
    }
}));

const renderDropdown = (appName = 'app-one') => {
    const navigation = {goto: jest.fn(), history: {} as any};
    const ctx = {
        navigation,
        popup: {} as any,
        notifications: {} as any,
        baseHref: '/',
        history: {} as any
    };
    let tree: renderer.ReactTestRenderer;
    renderer.act(() => {
        tree = renderer.create(
            <Context.Provider value={ctx as any}>
                <KeybindingProvider>
                    <ApplicationsDetailsAppDropdown appName={appName} objectListKind='Application' />
                </KeybindingProvider>
            </Context.Provider>
        );
    });
    return {tree: tree!, navigation};
};

const findByClass = (node: renderer.ReactTestInstance, className: string) =>
    node.findAll(n => typeof n.type === 'string' && typeof n.props.className === 'string' && (n.props.className as string).split(' ').includes(className));

describe('ApplicationsDetailsAppDropdown', () => {
    it('renders the current app name in the anchor and keeps the panel closed by default', () => {
        const {tree} = renderDropdown('app-one');
        const anchor = findByClass(tree.root, 'application-details-app-dropdown__anchor');
        expect(anchor).toHaveLength(1);
        expect(JSON.stringify(tree.toJSON())).toContain('app-one');
        expect(findByClass(tree.root, 'application-details-app-dropdown__panel')).toHaveLength(0);
    });

    it('opens the panel and queries the applications service when the anchor is clicked', () => {
        const {services} = require('../../../shared/services');
        (services.applications.list as jest.Mock).mockClear();
        const {tree} = renderDropdown();
        const anchor = findByClass(tree.root, 'application-details-app-dropdown__anchor')[0];
        renderer.act(() => {
            (anchor.props.onClick as () => void)();
        });
        expect(findByClass(tree.root, 'application-details-app-dropdown__panel')).toHaveLength(1);
        expect(services.applications.list).toHaveBeenCalledTimes(1);
        expect(services.applications.list).toHaveBeenCalledWith([], 'Application', {fields: ['items.metadata.name', 'items.metadata.namespace']});
    });

    it('closes the panel when the anchor is clicked again', () => {
        const {tree} = renderDropdown();
        const anchor = findByClass(tree.root, 'application-details-app-dropdown__anchor')[0];
        renderer.act(() => (anchor.props.onClick as () => void)());
        expect(findByClass(tree.root, 'application-details-app-dropdown__panel')).toHaveLength(1);
        renderer.act(() => (anchor.props.onClick as () => void)());
        expect(findByClass(tree.root, 'application-details-app-dropdown__panel')).toHaveLength(0);
    });
});
