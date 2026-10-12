import * as React from 'react';
import * as models from '../../../shared/models';
import {getAppDefaultSource, isApp} from '../utils';

export interface WindowScrollerHandle {
    updatePosition: () => void;
}

export function useWindowScrollerPosition(scrollerRef: React.RefObject<WindowScrollerHandle | null>, enabled: boolean, layoutKey: unknown): void {
    React.useLayoutEffect(() => {
        if (enabled) {
            scrollerRef.current?.updatePosition();
        }
    }, [enabled, layoutKey, scrollerRef]);
}

/** Virtualize only when "Items per page: all" is selected and list is greater than this. */
export const VIRTUAL_THRESHOLD = 50;

/** Extra rows above/below the viewport (low values flash blank on fast scroll). */
export const TILE_OVERSCAN_ROW_COUNT = 8;
export const TABLE_OVERSCAN_ROW_COUNT = 16;

/**
 * Virtual table slot heights include the 8px gap used by argo-ui table rows.
 * The row itself needs 60px (86px with a hydrator status line).
 */
export const TABLE_ROW_HEIGHT = 68;

/** Table row height when a source-hydrator status line is present. */
export const TABLE_ROW_HEIGHT_WITH_HYDRATOR = 94;

/** Default tile height until CellMeasurer measures the cell (excludes TILE_GAP). */
export const TILE_HEIGHT = 360;

/** Matches applications-tiles.scss gap / minmax(370px, 1fr). */
export const TILE_GAP = 24;
export const TILE_MIN_WIDTH = 370;

export function shouldUseVirtualScroll(useVirtualScrolling: boolean | undefined, length: number): boolean {
    return !!useVirtualScrolling && length > VIRTUAL_THRESHOLD;
}

export function computeColumnsPerRow(containerWidth: number, minItemWidth: number = TILE_MIN_WIDTH, gap: number = TILE_GAP): number {
    return Math.max(1, Math.floor((containerWidth + gap) / (minItemWidth + gap)));
}

export function computeColumnWidth(containerWidth: number, columnsPerRow: number, gap: number = TILE_GAP): number {
    return Math.floor((containerWidth - (columnsPerRow - 1) * gap) / columnsPerRow);
}

/** Last column is content-only; others include trailing gap (avoids horizontal scrollbar). */
export function computeColumnWidthForIndex(containerWidth: number, columnsPerRow: number, columnIndex: number, gap: number = TILE_GAP): number {
    const tileWidth = computeColumnWidth(containerWidth, columnsPerRow, gap);
    if (columnIndex >= columnsPerRow - 1) {
        return tileWidth;
    }
    return tileWidth + gap;
}

export function hasActiveHydrator(app: models.AbstractApplication): boolean {
    return isApp(app) && !!(app as models.Application).status?.sourceHydrator?.currentOperation;
}

export function getTableRowHeight(app: models.AbstractApplication): number {
    if (hasActiveHydrator(app)) {
        return TABLE_ROW_HEIGHT_WITH_HYDRATOR;
    }
    return TABLE_ROW_HEIGHT;
}

/**
 * Bits for conditional Application tile rows that change measured height
 * (Path, Chart, Last Sync, hydrator). Matches ApplicationTile rendering.
 */
export function tileHeightBits(app: models.AbstractApplication): number {
    let bits = 0;
    if (hasActiveHydrator(app)) {
        bits |= 1;
    }
    if (!isApp(app)) {
        return bits;
    }
    const application = app as models.Application;
    const source = getAppDefaultSource(application);
    if (source?.path) {
        bits |= 2;
    }
    if (source?.chart) {
        bits |= 4;
    }
    if (application.status?.operationState) {
        bits |= 8;
    }
    return bits;
}

/** Hash of app ids + height-affecting tile rows. Changes on reorder or those rows appearing/disappearing. */
export function appsLayoutKey(apps: models.AbstractApplication[]): string {
    let hash = 2166136261;
    for (let i = 0; i < apps.length; i++) {
        const app = apps[i];
        const id = app.metadata?.uid || `${app.metadata?.namespace || ''}/${app.metadata?.name || ''}`;
        for (let j = 0; j < id.length; j++) {
            hash ^= id.charCodeAt(j);
            hash = Math.imul(hash, 16777619);
        }
        hash ^= tileHeightBits(app);
        hash = Math.imul(hash, 16777619);
    }
    return `${apps.length}:${hash >>> 0}`;
}
