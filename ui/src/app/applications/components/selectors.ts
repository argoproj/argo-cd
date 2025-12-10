import * as LabelSelector from './label-selector';
import {Application} from '../../shared/models';

export interface Selector {
    match(resource: Record<string, string> | undefined): boolean;
}

/**
 * Create a metadata selector that can be used for both labels and annotations
 * Reuses existing LabelSelector functionality including all operators (==, !=, in, notin, gt, lt)
 */
export const createMetadataSelector = (selectors: string[]) => {
    return (metadata: Record<string, string> | undefined): boolean => {
        return selectors.every(selector => LabelSelector.match(selector, metadata || {}));
    };
};

/**
 * Create a universal application filter that handles both labels and annotations
 * using the powerful LabelSelector functionality for both
 */
export const createAppFilter = (pref: any) => {
    const labelSelector = createMetadataSelector(pref.labelsFilter || []);
    const annotationSelector = createMetadataSelector(pref.annotationsFilter || []);

    return (app: Application): boolean => {
        return labelSelector(app.metadata.labels) && annotationSelector(app.metadata.annotations);
    };
};

/**
 * Backward compatibility: maintain existing LabelSelector functionality
 */
export {LabelSelector};

/**
 * Test cases for annotation filtering
 * These cases should all pass:
 *
 * annotations    filter    expected
 * {team: core}    ["team=core"]    PASS
 * {team: core}    ["team"]    PASS
 * {team: core}    ["team=ops"]    FAIL
 * {}    ["team"]    FAIL
 * {version: "v2"}    ["version=v2"]    PASS
 */
