import * as models from '../../../shared/models';
import {nodeKey} from '../utils';

// selectedResource is the `deploy` query param that opened the sync panel: 'all', a
// single resource key, or a comma-separated list of keys (from "Sync selected" in the
// diff panel). Returns the initial checkbox state. A single value matching no resource
// (like 'all') keeps the existing rule and selects everything. A list matching no
// resource selects nothing, so a stale selection never turns into a full sync.
export function resourcesPreSelection(appResources: models.ResourceStatus[], selectedResource: string): boolean[] {
    const value = selectedResource || '';
    const keys = new Set(value.split(',').filter(key => key !== ''));
    const anyMatch = appResources.some(item => keys.has(nodeKey(item)));
    if (!anyMatch && value.includes(',')) {
        return appResources.map(() => false);
    }
    return appResources.map(item => !anyMatch || keys.has(nodeKey(item)));
}
