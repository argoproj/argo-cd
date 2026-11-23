import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, renderHook, waitFor} from '@testing-library/react';
import * as React from 'react';
import {Observable, Subject} from 'rxjs';

import {Application} from '../models';
import {applyServerSideWatchEvents} from '../../applications/components/applications-list/applications-list-client';
import {applyWatchEvents, useWatchedList, WATCH_REFETCH_DEBOUNCE_MS, WatchEvent} from './use-watched-list';

const app = (name: string, health = 'Healthy', resourceVersion = '1') =>
    ({metadata: {name, namespace: 'argocd', resourceVersion}, status: {health: {status: health}, sync: {status: 'Synced'}}, spec: {}}) as unknown as Application;

test('applyWatchEvents adds, replaces and removes by instance name without mutating its input', () => {
    const list = {items: [app('a'), app('b')], metadata: {resourceVersion: '1'}} as any;
    const updated = applyWatchEvents(list, [
        {type: 'ADDED', application: app('c')},
        {type: 'MODIFIED', application: app('a', 'Degraded')},
        {type: 'DELETED', application: app('b')}
    ]);
    expect(updated.items.map((item: Application) => [item.metadata.name, item.status.health.status])).toEqual([
        ['c', 'Healthy'],
        ['a', 'Degraded']
    ]);
    expect(updated.metadata.resourceVersion).toBe('1');
    expect(list.items).toHaveLength(2);
});

test('applyServerSideWatchEvents refetches on matching ADDED without inserting the row', () => {
    const list = {items: [app('a')], metadata: {resourceVersion: '1'}, stats: {total: 1}} as any;
    const matches = (name: string) => name.startsWith('a') || name === 'c';
    const result = applyServerSideWatchEvents(list, [{type: 'ADDED', application: app('c')}], matches);
    expect(result.list.items.map((item: Application) => item.metadata.name)).toEqual(['a']);
    expect(result.refetch).toBe(true);
});

test('applyServerSideWatchEvents ignores non-matching ADDED without refetch', () => {
    const list = {items: [app('a')], metadata: {resourceVersion: '1'}, stats: {total: 1}} as any;
    const matches = (name: string) => name === 'a';
    const result = applyServerSideWatchEvents(list, [{type: 'ADDED', application: app('other')}], matches);
    expect(result.list.items.map((item: Application) => item.metadata.name)).toEqual(['a']);
    expect(result.refetch).toBeFalsy();
});

test('applyServerSideWatchEvents ignores MODIFIED for apps not on the current page', () => {
    const list = {items: [app('a')], metadata: {resourceVersion: '1'}} as any;
    const matches = () => true;
    const result = applyServerSideWatchEvents(list, [{type: 'MODIFIED', application: app('other', 'Degraded')}], matches);
    expect(result.list.items.map((item: Application) => item.metadata.name)).toEqual(['a']);
    expect(result.refetch).toBeFalsy();
});

test('applyServerSideWatchEvents updates in place, drops apps that leave search, and refetches when aggregate fields change', () => {
    const list = {items: [app('a'), app('ab')], metadata: {resourceVersion: '1'}} as any;
    const matches = (name: string) => name.startsWith('a') && name !== 'ab';
    const result = applyServerSideWatchEvents(
        list,
        [
            {type: 'MODIFIED', application: app('a', 'Degraded', '2')},
            {type: 'MODIFIED', application: app('ab', 'Healthy', '2')}
        ],
        matches
    );
    expect(result.list.items.map((item: Application) => [item.metadata.name, item.status.health.status])).toEqual([['a', 'Degraded']]);
    expect(result.refetch).toBe(true);
});

test('applyServerSideWatchEvents skips MODIFIED events older than the installed snapshot', () => {
    const list = {items: [app('a', 'Healthy', '5')], metadata: {resourceVersion: '1'}} as any;
    const matches = () => true;
    const result = applyServerSideWatchEvents(list, [{type: 'MODIFIED', application: app('a', 'Degraded', '3')}], matches);
    expect(result.list.items[0].status.health.status).toBe('Healthy');
    expect(result.refetch).toBeFalsy();
});

test('applyServerSideWatchEvents replaces newer MODIFIED without stats refetch when aggregate fields are unchanged', () => {
    const list = {items: [app('a', 'Healthy', '5')], metadata: {resourceVersion: '1'}} as any;
    const matches = () => true;
    const newer = app('a', 'Healthy', '6');
    (newer.metadata as any).annotations = {tick: '1'};
    const result = applyServerSideWatchEvents(list, [{type: 'MODIFIED', application: newer}], matches);
    expect(result.list.items[0].metadata.resourceVersion).toBe('6');
    expect(result.refetch).toBeFalsy();
});

test('applyServerSideWatchEvents removes DELETED on-page apps and requests a refetch to refill', () => {
    const list = {items: [app('a'), app('b')], metadata: {resourceVersion: '1'}} as any;
    const matches = () => true;
    const result = applyServerSideWatchEvents(list, [{type: 'DELETED', application: app('a')}], matches);
    expect(result.list.items.map((item: Application) => item.metadata.name)).toEqual(['b']);
    expect(result.refetch).toBe(true);
});

test('applyServerSideWatchEvents refetches on off-page DELETE without inserting foreign rows', () => {
    // Off-page deletes can shift which apps belong in the current offset/limit window and change totals.
    const list = {items: [app('a')], metadata: {resourceVersion: '1'}, stats: {total: 5}} as any;
    const matches = () => true;
    const result = applyServerSideWatchEvents(
        list,
        [
            {type: 'DELETED', application: app('other')},
            {type: 'DELETED', application: app('also-off-page')}
        ],
        matches
    );
    expect(result.list.items.map((item: Application) => item.metadata.name)).toEqual(['a']);
    expect(result.list.items.some((item: Application) => item.metadata.name === 'other')).toBe(false);
    expect(result.refetch).toBe(true);
});

test('useWatchedList coalesces off-page DELETE refetches into one invalidation', async () => {
    jest.useFakeTimers();
    const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const invalidateSpy = jest.spyOn(queryClient, 'invalidateQueries');
    const watch$ = new Subject<WatchEvent<Application>>();
    const initial = {items: [app('a')], metadata: {resourceVersion: '1'}, stats: {total: 5}} as any;

    const wrapper = ({children}: {children: React.ReactNode}) => React.createElement(QueryClientProvider, {client: queryClient}, children);

    renderHook(
        () =>
            useWatchedList(
                {
                    queryKey: ['applications', 'list', 'view', 'coalesce-test'],
                    queryFn: async () => initial
                },
                () => watch$ as Observable<WatchEvent<Application>>,
                {
                    watchKey: 'stable',
                    applyEvents: (list, changes) => applyServerSideWatchEvents(list, changes, () => true)
                }
            ),
        {wrapper}
    );

    await waitFor(() => expect(queryClient.getQueryData(['applications', 'list', 'view', 'coalesce-test'])).toBeTruthy());
    invalidateSpy.mockClear();

    act(() => {
        watch$.next({type: 'DELETED', application: app('other')});
        watch$.next({type: 'DELETED', application: app('also-off-page')});
    });

    // bufferTime(500) then WATCH_REFETCH_DEBOUNCE_MS — advance past both.
    act(() => {
        jest.advanceTimersByTime(500);
    });
    const cached = queryClient.getQueryData(['applications', 'list', 'view', 'coalesce-test']) as any;
    expect(cached.items.map((item: Application) => item.metadata.name)).toEqual(['a']);
    expect(cached.items.some((item: Application) => item.metadata.name === 'other')).toBe(false);

    act(() => {
        jest.advanceTimersByTime(WATCH_REFETCH_DEBOUNCE_MS);
    });
    expect(invalidateSpy).toHaveBeenCalledTimes(1);

    jest.useRealTimers();
    queryClient.clear();
});
