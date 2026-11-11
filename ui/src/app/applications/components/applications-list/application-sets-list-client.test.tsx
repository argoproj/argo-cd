import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, renderHook, waitFor} from '@testing-library/react';
import * as React from 'react';
import {Subject} from 'rxjs';

import {ApplicationSet, ApplicationWatchEvent} from '../../../shared/models';
import {services} from '../../../shared/services';
import {useApplicationSetsList} from './application-sets-list-client';

jest.mock('../../../shared/services', () => {
    const actual = jest.requireActual('../../../shared/services');
    return {...actual, services: {...actual.services, applications: {list: jest.fn(), watch: jest.fn()}}};
});

const appSet = (name: string, health = 'Healthy') => ({metadata: {name, namespace: 'argocd'}, spec: {}, status: {health: {status: health}}}) as unknown as ApplicationSet;

// The service normalizes ApplicationSet watch events to the application field.
const event = (type: ApplicationWatchEvent['type'], item: ApplicationSet) => ({type, application: item}) as unknown as ApplicationWatchEvent;

let watch: Subject<ApplicationWatchEvent>;

const setup = () => {
    const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const wrapper = ({children}: {children: React.ReactNode}) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
    return renderHook(() => useApplicationSetsList(), {wrapper}).result;
};

const setHidden = (hidden: boolean) => {
    Object.defineProperty(document, 'hidden', {configurable: true, get: () => hidden});
    document.dispatchEvent(new Event('visibilitychange'));
};

beforeEach(() => {
    jest.clearAllMocks();
    watch = new Subject<ApplicationWatchEvent>();
    jest.mocked(services.applications.watch).mockReturnValue(watch);
    jest.mocked(services.applications.list).mockResolvedValue({items: [appSet('a'), appSet('b')], metadata: {resourceVersion: '5'}} as any);
});

test('loads the list and applies watch events from the version it was read at', async () => {
    const result = setup();
    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(services.applications.watch).toHaveBeenCalled());
    expect(result.current.appSets.map(item => item.metadata.name)).toEqual(['a', 'b']);
    expect(jest.mocked(services.applications.watch).mock.calls[0].slice(0, 2)).toEqual(['applicationset', {resourceVersion: '5'}]);

    act(() => {
        watch.next(event('ADDED', appSet('c')));
        watch.next(event('MODIFIED', appSet('b', 'Degraded')));
        watch.next(event('DELETED', appSet('a')));
    });

    await waitFor(() =>
        expect(result.current.appSets.map(item => [item.metadata.name, item.status.health.status])).toEqual([
            ['c', 'Healthy'],
            ['b', 'Degraded']
        ])
    );
});

test('reports a failed load and retries it', async () => {
    jest.mocked(services.applications.list).mockRejectedValueOnce(new Error('boom'));
    const result = setup();
    await waitFor(() => expect(result.current.failed).toBe(true));
    expect(services.applications.watch).not.toHaveBeenCalled();

    act(() => {
        result.current.retry();
    });

    await waitFor(() => expect(result.current.appSets).toHaveLength(2));
    expect(result.current.failed).toBe(false);
});

test('reloads before watching again after being hidden, and reports a failed reload instead of resuming the watch', async () => {
    try {
        const result = setup();
        await waitFor(() => expect(services.applications.watch).toHaveBeenCalledTimes(1));

        setHidden(true);
        await waitFor(() => expect(watch.observed).toBe(false), {timeout: 2000});

        jest.mocked(services.applications.list).mockRejectedValueOnce(new Error('boom'));
        setHidden(false);
        await waitFor(() => expect(result.current.failed).toBe(true), {timeout: 2000});
        expect(services.applications.watch).toHaveBeenCalledTimes(1);

        jest.mocked(services.applications.list).mockResolvedValue({items: [appSet('a')], metadata: {resourceVersion: '9'}} as any);
        act(() => {
            result.current.retry();
        });
        await waitFor(() => expect(services.applications.watch).toHaveBeenCalledTimes(2));
        expect(jest.mocked(services.applications.watch).mock.calls[1][1]).toEqual({resourceVersion: '9'});
        expect(result.current.failed).toBe(false);
        expect(result.current.appSets).toHaveLength(1);
    } finally {
        setHidden(false);
    }
});
