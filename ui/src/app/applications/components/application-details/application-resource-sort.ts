import * as models from '../../../shared/models';
import {HealthPriority, SyncPriority, SyncStatusCode} from '../../../shared/models';
import {createdOrNodeKey} from '../utils';

export type ApplicationResourceSortKey = 'name' | 'group-kind' | 'syncOrder' | 'namespace' | 'createdAt' | 'status';

export const APPLICATION_DETAILS_SORT_KEY = 'application-details';
export const GROUPED_NODES_DETAILS_SORT_KEY = 'grouped-nodes-details';

// Direction-agnostic comparison for the application resource list columns. Callers apply the
// direction (e.g. multiply by the `dir` returned from useListSort).
export function compareApplicationResource(a: models.ResourceStatus, b: models.ResourceStatus, key: ApplicationResourceSortKey): number {
    switch (key) {
        case 'name':
            return a.name.localeCompare(b.name);
        case 'group-kind': {
            const ga = [a.group, a.kind].filter(Boolean).join('/');
            const gb = [b.group, b.kind].filter(Boolean).join('/');
            return ga.localeCompare(gb);
        }
        case 'syncOrder':
            return (a.syncWave ?? 0) - (b.syncWave ?? 0);
        case 'namespace':
            return (a.namespace ?? '').localeCompare(b.namespace ?? '');
        case 'createdAt':
            return createdOrNodeKey(a).localeCompare(createdOrNodeKey(b), undefined, {numeric: true});
        case 'status': {
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
        default:
            return 0;
    }
}
