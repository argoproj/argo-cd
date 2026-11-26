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
 * Builds a Kubernetes label selector from the selectors that the API server matches the same way as
 * createMetadataSelector. The result may match more than the client does, never less, so the client
 * still has to apply all of the selectors.
 */
export const toServerLabelSelector = (selectors: string[]): string =>
    selectors
        .map(LabelSelector.toKubernetesSelector)
        .filter(selector => !!selector)
        .join(',');

/**
 * Backward compatibility: maintain existing LabelSelector functionality
 */
export {LabelSelector};
