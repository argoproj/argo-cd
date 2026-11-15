import * as models from '../../../shared/models';

export type SyncStatusFilter = 'all' | 'outOfSync' | 'none';

export function getAvailableKinds(resources: models.ResourceStatus[]): string[] {
    return Array.from(new Set(resources.map(resource => resource.kind)))
        .filter(Boolean)
        .sort((a, b) => a.localeCompare(b));
}

export function filterResources(
    resources: {item: models.ResourceStatus; i: number}[],
    kindFilter: string,
    statusFilter: SyncStatusFilter
): {item: models.ResourceStatus; i: number}[] {
    return resources.filter(({item}) => {
        if (kindFilter !== '' && item.kind !== kindFilter) {
            return false;
        }
        if (statusFilter === 'outOfSync' && item.status !== models.SyncStatuses.OutOfSync) {
            return false;
        }
        return true;
    });
}

export function computeSelection(resources: models.ResourceStatus[], kindFilter: string, statusFilter: SyncStatusFilter): boolean[] {
    if (statusFilter === 'none') {
        return resources.map(() => false);
    }
    return resources.map(resource => {
        if (kindFilter !== '' && resource.kind !== kindFilter) {
            return false;
        }
        if (statusFilter === 'outOfSync') {
            return resource.status === models.SyncStatuses.OutOfSync;
        }
        return true;
    });
}
