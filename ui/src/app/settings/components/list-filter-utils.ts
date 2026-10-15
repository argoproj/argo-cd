import * as React from 'react';

export interface FilterOption {
    label: string;
    icon?: React.ReactNode;
    count?: number;
}

export const capitalize = (s: string): string => s.charAt(0).toUpperCase() + s.slice(1);

// Items carry a `filterResult` map of dimension -> whether the item passes that dimension's filter.
// The concrete FilterResult interfaces have no index signature, so read it through this shape.
type WithFilterResult = {filterResult: Record<string, boolean>};

// Count items per value for a single filter dimension. A dimension's own selection is ignored (so
// its counts reflect what would match if each value were toggled), while every other active
// dimension still applies. `init` seeds zero counts so known values render even when nothing matches.
export function getFilterCounts<T>(items: T[], filterType: string, getValue: (item: T) => string | string[] | undefined, init?: string[]): Map<string, number> {
    const map = new Map<string, number>();
    if (init) {
        init.forEach(key => map.set(key, 0));
    }
    const bump = (val: string) => map.set(val, (map.get(val) || 0) + 1);
    items.forEach(item => {
        const filterResult = (item as unknown as WithFilterResult).filterResult;
        if (Object.keys(filterResult).every(key => key === filterType || filterResult[key])) {
            const val = getValue(item);
            if (Array.isArray(val)) {
                val.forEach(bump);
            } else if (val !== undefined) {
                bump(val);
            }
        }
    });
    return map;
}

export function getFilterOptions<T>(
    items: T[],
    filterType: string,
    getValue: (item: T) => string | string[] | undefined,
    keys: string[],
    getIcon?: (k: string) => React.ReactNode,
    getLabel?: (k: string) => string
): FilterOption[] {
    const counts = getFilterCounts(items, filterType, getValue, keys);
    return keys.map(k => ({
        label: getLabel ? getLabel(k) : k,
        icon: getIcon && getIcon(k),
        count: counts.get(k)
    }));
}
