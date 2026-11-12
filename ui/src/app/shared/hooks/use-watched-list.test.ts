import {Application} from '../models';
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
