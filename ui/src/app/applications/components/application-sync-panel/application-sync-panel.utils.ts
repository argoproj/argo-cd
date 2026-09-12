import * as models from '../../../shared/models';

export function getAvailableKinds(resources: models.ResourceStatus[]): string[] {
    return Array.from(new Set(resources.map(resource => resource.kind)))
        .filter(Boolean)
        .sort((a, b) => a.localeCompare(b));
}

export function setSelectionForKind(resources: models.ResourceStatus[], selections: boolean[], kind: string, value: boolean): boolean[] {
    return resources.map((resource, i) => (resource.kind === kind ? value : selections[i]));
}
