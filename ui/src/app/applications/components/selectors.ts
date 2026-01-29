import * as LabelSelector from './label-selector';

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
 * Backward compatibility: maintain existing LabelSelector functionality
 */
export {LabelSelector};
