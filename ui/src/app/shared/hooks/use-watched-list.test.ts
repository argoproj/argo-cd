import {Application} from '../models';
import {applyServerSideWatchEvents} from '../../applications/components/applications-list/applications-list-client';
import {applyWatchEvents} from './use-watched-list';

const app = (name: string, health = 'Healthy') => ({metadata: {name, namespace: 'argocd'}, status: {health: {status: health}}}) as unknown as Application;

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

test('applyServerSideWatchEvents ignores ADDED without unshifting or refetching', () => {
    const list = {items: [app('a')], metadata: {resourceVersion: '1'}, stats: {total: 1}} as any;
    const matches = (name: string) => name.startsWith('a') || name === 'c';
    const result = applyServerSideWatchEvents(list, [{type: 'ADDED', application: app('c')}], matches);
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

test('applyServerSideWatchEvents updates in place and drops apps that leave the search without refetch', () => {
    const list = {items: [app('a'), app('ab')], metadata: {resourceVersion: '1'}} as any;
    const matches = (name: string) => name.startsWith('a') && name !== 'ab';
    const result = applyServerSideWatchEvents(
        list,
        [
            {type: 'MODIFIED', application: app('a', 'Degraded')},
            {type: 'MODIFIED', application: app('ab')}
        ],
        matches
    );
    expect(result.list.items.map((item: Application) => [item.metadata.name, item.status.health.status])).toEqual([['a', 'Degraded']]);
    expect(result.refetch).toBeFalsy();
});

test('applyServerSideWatchEvents removes DELETED on-page apps and requests a refetch to refill', () => {
    const list = {items: [app('a'), app('b')], metadata: {resourceVersion: '1'}} as any;
    const matches = () => true;
    const result = applyServerSideWatchEvents(list, [{type: 'DELETED', application: app('a')}], matches);
    expect(result.list.items.map((item: Application) => item.metadata.name)).toEqual(['b']);
    expect(result.refetch).toBe(true);
});

test('applyServerSideWatchEvents ignores DELETED for apps not on the page', () => {
    const list = {items: [app('a')], metadata: {resourceVersion: '1'}} as any;
    const matches = () => true;
    const result = applyServerSideWatchEvents(list, [{type: 'DELETED', application: app('other')}], matches);
    expect(result.list.items.map((item: Application) => item.metadata.name)).toEqual(['a']);
    expect(result.refetch).toBeFalsy();
});
