import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {BehaviorSubject, Subscription} from 'rxjs';

import {ContextApis} from '../../shared/context';
import {AbstractApplication, Application, ApplicationTree, ResourceNode} from '../../shared/models';
import {services} from '../../shared/services';
import {renderResourceButtons} from './utils';

describe('renderResourceButtons', () => {
    const resource = {
        uid: 'apps/Deployment/default/guestbook',
        group: 'apps',
        kind: 'Deployment',
        namespace: 'default',
        name: 'guestbook'
    } as ResourceNode;

    const otherResource = {
        ...resource,
        uid: 'apps/Deployment/default/guestbook-canary',
        name: 'guestbook-canary'
    } as ResourceNode;

    const application = {
        kind: 'Application',
        metadata: {name: 'guestbook', namespace: 'argocd'},
        spec: {project: 'default'},
        status: {
            resources: [
                {
                    group: resource.group,
                    kind: resource.kind,
                    namespace: resource.namespace,
                    name: resource.name
                }
            ]
        }
    } as Application;

    const tree = {
        nodes: [resource]
    } as ApplicationTree;

    const apis = {
        popup: {prompt: jest.fn()},
        notifications: {show: jest.fn()},
        navigation: {goto: jest.fn()},
        baseHref: ''
    } as unknown as ContextApis;

    afterEach(() => {
        jest.restoreAllMocks();
    });

    function QuickStartWrapper(props: {resourceNode: ResourceNode; version: number; appChanged: BehaviorSubject<AbstractApplication>}) {
        return <div data-version={props.version}>{renderResourceButtons(props.resourceNode, application, tree, apis, props.appChanged)}</div>;
    }

    it('does not reload quickstart actions on unrelated parent rerenders', () => {
        const canISpy = jest.spyOn(services.accounts, 'canI').mockResolvedValue(false);
        const appChanged = new BehaviorSubject<AbstractApplication>(application);
        const subscription: Subscription = appChanged.subscribe(() => undefined);
        let rendered: renderer.ReactTestRenderer | undefined;

        try {
            renderer.act(() => {
                rendered = renderer.create(<QuickStartWrapper resourceNode={resource} version={0} appChanged={appChanged} />);
            });

            expect(canISpy).toHaveBeenCalledTimes(1);

            renderer.act(() => {
                rendered!.update(<QuickStartWrapper resourceNode={resource} version={1} appChanged={appChanged} />);
            });

            expect(canISpy).toHaveBeenCalledTimes(1);
        } finally {
            subscription.unsubscribe();
        }
    });

    it('reloads quickstart actions when the resource changes', () => {
        const canISpy = jest.spyOn(services.accounts, 'canI').mockResolvedValue(false);
        const appChanged = new BehaviorSubject<AbstractApplication>(application);
        const subscription: Subscription = appChanged.subscribe(() => undefined);
        let rendered: renderer.ReactTestRenderer | undefined;

        try {
            renderer.act(() => {
                rendered = renderer.create(<QuickStartWrapper resourceNode={resource} version={0} appChanged={appChanged} />);
            });

            renderer.act(() => {
                rendered!.update(<QuickStartWrapper resourceNode={otherResource} version={0} appChanged={appChanged} />);
            });

            expect(canISpy).toHaveBeenCalledTimes(2);
        } finally {
            subscription.unsubscribe();
        }
    });

    it('reloads quickstart actions when the appChanged subject identity changes', () => {
        const canISpy = jest.spyOn(services.accounts, 'canI').mockResolvedValue(false);
        const firstAppChanged = new BehaviorSubject<AbstractApplication>(application);
        const secondAppChanged = new BehaviorSubject<AbstractApplication>(application);
        const firstSubscription: Subscription = firstAppChanged.subscribe(() => undefined);
        const secondSubscription: Subscription = secondAppChanged.subscribe(() => undefined);
        let rendered: renderer.ReactTestRenderer | undefined;

        try {
            renderer.act(() => {
                rendered = renderer.create(<QuickStartWrapper resourceNode={resource} version={0} appChanged={firstAppChanged} />);
            });

            renderer.act(() => {
                rendered!.update(<QuickStartWrapper resourceNode={resource} version={0} appChanged={secondAppChanged} />);
            });

            expect(canISpy).toHaveBeenCalledTimes(2);
        } finally {
            firstSubscription.unsubscribe();
            secondSubscription.unsubscribe();
        }
    });
});
