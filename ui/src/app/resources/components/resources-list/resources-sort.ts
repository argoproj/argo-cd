import * as models from '../../../shared/models';
import {HealthPriority, SyncPriority, SyncStatusCode} from '../../../shared/models';
import {SortOption} from '../../../shared/components';
import {resourceHealthStatus} from '../utils';

export type ResourceSortKey = 'name' | 'group-kind' | 'namespace' | 'cluster' | 'application' | 'status';

export const RESOURCES_LIST_SORT_KEY = 'resources-list';

export const RESOURCE_SORT_KEY_TO_TITLE: Record<ResourceSortKey, string> = {
    'name': 'Name',
    'group-kind': 'Group/Kind',
    'namespace': 'Namespace',
    'cluster': 'Cluster',
    'application': 'Application',
    'status': 'Status'
};

export const RESOURCE_SORT_TITLE_TO_KEY = Object.fromEntries(Object.entries(RESOURCE_SORT_KEY_TO_TITLE).map(([key, title]) => [title, key])) as Record<string, ResourceSortKey>;

export const RESOURCE_SORT_OPTIONS: SortOption<models.Resource>[] = [
    {title: 'Name', compare: (a, b) => a.name.localeCompare(b.name)},
    {
        title: 'Group/Kind',
        compare: (a, b) => {
            const ga = [a.group, a.kind].filter(Boolean).join('/');
            const gb = [b.group, b.kind].filter(Boolean).join('/');
            return ga.localeCompare(gb);
        }
    },
    {title: 'Namespace', compare: (a, b) => (a.namespace || '').localeCompare(b.namespace || '')},
    {
        title: 'Cluster',
        compare: (a, b) => {
            const ca = a.clusterName || a.clusterServer || '';
            const cb = b.clusterName || b.clusterServer || '';
            return ca.localeCompare(cb);
        }
    },
    {title: 'Application', compare: (a, b) => (a.appName || '').localeCompare(b.appName || '')},
    {
        title: 'Status',
        compare: (a, b) => {
            const healthA = resourceHealthStatus(a);
            const healthB = resourceHealthStatus(b);
            const syncA = (a.status as SyncStatusCode) ?? 'Unknown';
            const syncB = (b.status as SyncStatusCode) ?? 'Unknown';
            let compare = HealthPriority[healthA] - HealthPriority[healthB];
            if (compare === 0) {
                compare = SyncPriority[syncA] - SyncPriority[syncB];
            }
            return compare;
        }
    }
];
