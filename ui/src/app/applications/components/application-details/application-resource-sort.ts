import * as models from '../../../shared/models';
import {HealthPriority, SyncPriority, SyncStatusCode} from '../../../shared/models';
import {SortOption} from '../../../shared/components';
import {createdOrNodeKey} from '../utils';

export type ApplicationResourceSortKey = 'name' | 'group-kind' | 'syncOrder' | 'namespace' | 'createdAt' | 'status';

export const APPLICATION_DETAILS_SORT_KEY = 'application-details';
export const GROUPED_NODES_DETAILS_SORT_KEY = 'grouped-nodes-details';

export const APPLICATION_RESOURCE_SORT_KEY_TO_TITLE: Record<ApplicationResourceSortKey, string> = {
    'name': 'Name',
    'group-kind': 'Group/Kind',
    'syncOrder': 'Sync Order',
    'namespace': 'Namespace',
    'createdAt': 'Created At',
    'status': 'Status'
};

export const APPLICATION_RESOURCE_SORT_TITLE_TO_KEY = Object.fromEntries(Object.entries(APPLICATION_RESOURCE_SORT_KEY_TO_TITLE).map(([key, title]) => [title, key])) as Record<
    string,
    ApplicationResourceSortKey
>;

export const APPLICATION_RESOURCE_SORT_OPTIONS: SortOption<models.ResourceStatus>[] = [
    {
        title: 'Created At',
        defaultDirection: 'desc',
        compare: (a, b) => createdOrNodeKey(a).localeCompare(createdOrNodeKey(b), undefined, {numeric: true})
    },
    {title: 'Name', compare: (a, b) => a.name.localeCompare(b.name)},
    {
        title: 'Group/Kind',
        compare: (a, b) => {
            const ga = [a.group, a.kind].filter(Boolean).join('/');
            const gb = [b.group, b.kind].filter(Boolean).join('/');
            return ga.localeCompare(gb);
        }
    },
    {title: 'Sync Order', compare: (a, b) => (a.syncWave ?? 0) - (b.syncWave ?? 0)},
    {title: 'Namespace', compare: (a, b) => (a.namespace ?? '').localeCompare(b.namespace ?? '')},
    {
        title: 'Status',
        compare: (a, b) => {
            const healthA = a.health?.status ?? 'Unknown';
            const healthB = b.health?.status ?? 'Unknown';
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
