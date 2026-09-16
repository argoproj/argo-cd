import * as React from 'react';

// useListSort provides reusable client-side sorting state and helpers for the settings list tables.
// It tracks the active sort column and direction, and exposes:
//   - requestSort(key): click handler for a column header (toggles direction when the same key is clicked).
//   - sortIcon(key): the caret indicator for the active column (nothing for inactive columns).
//   - compareString(x, y): direction-aware string compare that treats empty values as the largest
//     (so they sort last ascending and first descending).
//   - compareNumber(x, y): direction-aware numeric/boolean compare.
export function useListSort<K extends string>(defaultKey: K, defaultAsc = true) {
    const [sortKey, setSortKey] = React.useState<K>(defaultKey);
    const [sortAsc, setSortAsc] = React.useState(defaultAsc);

    const requestSort = (key: K) => {
        if (sortKey === key) {
            setSortAsc(!sortAsc);
        } else {
            setSortKey(key);
            setSortAsc(true);
        }
    };

    const sortIcon = (key: K) => (sortKey === key ? <i className={`fa fa-caret-${sortAsc ? 'up' : 'down'}`} style={{marginLeft: '5px'}} /> : null);

    const dir = sortAsc ? 1 : -1;

    const compareString = (x: string, y: string) => {
        const a = x || '';
        const b = y || '';
        if (!a && !b) {
            return 0;
        }
        if (!a) {
            return dir;
        }
        if (!b) {
            return -dir;
        }
        return dir * a.localeCompare(b);
    };

    const compareNumber = (x: number, y: number) => dir * (x - y);

    return {sortKey, sortAsc, dir, requestSort, sortIcon, compareString, compareNumber};
}
