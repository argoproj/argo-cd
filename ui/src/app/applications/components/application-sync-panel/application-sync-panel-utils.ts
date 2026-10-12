import * as models from '../../../shared/models';

export type SelectionLink = 'all' | 'outOfSync' | 'none';

export interface KindCount {
    kind: string;
    count: number;
}

// An empty kinds list means "no filter": every resource is visible.
export function isVisible(resource: models.ResourceStatus, kinds: string[]): boolean {
    return kinds.length === 0 || kinds.includes(resource.kind);
}

export function getKindCounts(resources: models.ResourceStatus[]): KindCount[] {
    const counts = new Map<string, number>();
    resources.forEach(({kind}) => counts.set(kind, (counts.get(kind) || 0) + 1));
    return Array.from(counts, ([kind, count]) => ({kind, count})).sort((a, b) => a.kind.localeCompare(b.kind));
}

// Applies an all / out of sync / none link to the visible rows only. Hidden rows keep their value.
// `selection` is positional: selection[i] belongs to resources[i].
export function selectionForLink(resources: models.ResourceStatus[], selection: boolean[], kinds: string[], link: SelectionLink): boolean[] {
    return resources.map((resource, i) => {
        if (!isVisible(resource, kinds)) {
            return selection[i];
        }
        return link === 'all' || (link === 'outOfSync' && resource.status === models.SyncStatuses.OutOfSync);
    });
}

export function countHiddenSelected(resources: models.ResourceStatus[], selection: boolean[], kinds: string[]): number {
    return resources.filter((resource, i) => selection[i] && !isVisible(resource, kinds)).length;
}

export function deselectHidden(resources: models.ResourceStatus[], selection: boolean[], kinds: string[]): boolean[] {
    return resources.map((resource, i) => isVisible(resource, kinds) && selection[i]);
}
