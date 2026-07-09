import * as models from '../../../shared/models';
import {nodeKey} from '../utils';

// selectedResource is the `deploy` query param that opened the sync panel: 'all', a
// single resource key, or a comma-separated list of keys (from "Sync selected" in the
// diff panel). Returns the initial checkbox state, keeping the existing rule that a
// value matching no resource (like 'all') selects everything.
export function resourcesPreSelection(appResources: models.ResourceStatus[], selectedResource: string): boolean[] {
    const keys = new Set((selectedResource || '').split(',').filter(key => key !== ''));
    const anyMatch = appResources.some(item => keys.has(nodeKey(item)));
    return appResources.map(item => !anyMatch || keys.has(nodeKey(item)));
}
