// A resource version that can be ordered: a decimal integer with no leading zero.
const COMPARABLE_RESOURCE_VERSION = /^[1-9][0-9]*$/;

export function resourceVersionOf(value: unknown): string | null {
    const resourceVersion = (value as {metadata?: {resourceVersion?: string}})?.metadata?.resourceVersion;
    return typeof resourceVersion === 'string' && resourceVersion !== '' ? resourceVersion : null;
}

// Whether the live resource is at least as new as the version a save returned.
//
// Kubernetes guarantees resource versions of a given resource type increase monotonically, so they
// can be ordered. They are arbitrary bitsize integers though, so they are compared as strings by
// length and then lexicographically, as the API concepts documentation prescribes. Parsing them as
// numbers would silently lose precision past 2^53. Extension API servers may serve versions that are
// not decimal integers at all; those can only be compared for equality, and the caller is expected to
// stop waiting on its own rather than wait forever.
export function hasReachedResourceVersion(live: unknown, target: string): boolean {
    const current = resourceVersionOf(live);
    if (!current) {
        return false;
    }
    if (current === target) {
        return true;
    }
    if (!COMPARABLE_RESOURCE_VERSION.test(current) || !COMPARABLE_RESOURCE_VERSION.test(target)) {
        return false;
    }
    if (current.length !== target.length) {
        return current.length > target.length;
    }
    return current > target;
}
