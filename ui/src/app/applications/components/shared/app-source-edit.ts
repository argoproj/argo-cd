import {FormApi} from 'react-form';
import * as models from '../../../shared/models';

/** Shared with application-create-panel and application-parameters/source-panel. */
export const APP_SOURCE_TYPES = new Array<{field: string; type: models.AppSourceType}>(
    {type: 'Helm', field: 'helm'},
    {type: 'Kustomize', field: 'kustomize'},
    {type: 'Directory', field: 'directory'},
    {type: 'Plugin', field: 'plugin'}
);

/**
 * Clears sibling source-type blocks (helm/kustomize/directory/plugin) when the user picks a type.
 * @param sourceIndex — when set, edits `spec.sources[index]`; otherwise `spec.source`.
 */
export function normalizeTypeFieldsForSource(formApi: FormApi, type: models.AppSourceType, sourceIndex?: number): void {
    const appToNormalize = formApi.getFormState().values as models.Application;
    if (sourceIndex === undefined) {
        const single = appToNormalize.spec.source as unknown as Record<string, unknown>;
        for (const item of APP_SOURCE_TYPES) {
            if (item.type !== type) {
                delete single[item.field];
            }
        }
    } else {
        const src = appToNormalize.spec.sources[sourceIndex];
        if (!src) {
            return;
        }
        const srcRec = src as unknown as Record<string, unknown>;
        for (const item of APP_SOURCE_TYPES) {
            if (item.type !== type) {
                delete srcRec[item.field];
            }
        }
    }
    formApi.setAllValues(appToNormalize);
}
