import * as models from '../../../shared/models';

export interface ProjectGrouping {
    collapsed: string[];
    counts: Map<string, number>;
    onToggle: (project: string) => void;
}

export interface ProjectHeadingRow {
    kind: 'heading';
    project: string;
    count: number;
    collapsed: boolean;
}

export interface ProjectAppRow {
    kind: 'app';
    app: models.AbstractApplication;
    /** Position among visible apps, used for keyboard selection. */
    index: number;
}

export type ProjectRow = ProjectHeadingRow | ProjectAppRow;

export interface ProjectTilesRow {
    kind: 'tiles';
    items: ProjectAppRow[];
}

export type ProjectTileRow = ProjectHeadingRow | ProjectTilesRow;

export const appProject = (app: models.AbstractApplication): string => app.spec?.project || 'default';

export function countByProject(apps: models.AbstractApplication[]): Map<string, number> {
    const counts = new Map<string, number>();
    for (const app of apps) {
        const project = appProject(app);
        counts.set(project, (counts.get(project) || 0) + 1);
    }
    return counts;
}

export function buildProjectRows(apps: models.AbstractApplication[], grouping?: Pick<ProjectGrouping, 'collapsed' | 'counts'>): ProjectRow[] {
    if (!grouping) {
        return apps.map((app, index) => ({kind: 'app', app, index}));
    }
    const groups = new Map<string, models.AbstractApplication[]>();
    for (const app of apps) {
        const project = appProject(app);
        if (!groups.has(project)) {
            groups.set(project, []);
        }
        groups.get(project).push(app);
    }
    const rows: ProjectRow[] = [];
    let index = 0;
    groups.forEach((groupApps, project) => {
        const collapsed = grouping.collapsed.includes(project);
        rows.push({kind: 'heading', project, count: grouping.counts.get(project) ?? groupApps.length, collapsed});
        if (!collapsed) {
            for (const app of groupApps) {
                rows.push({kind: 'app', app, index: index++});
            }
        }
    });
    return rows;
}

export function visibleApps(rows: ProjectRow[]): models.AbstractApplication[] {
    return rows.filter((row): row is ProjectAppRow => row.kind === 'app').map(row => row.app);
}

export function buildTileRows(rows: ProjectRow[], columnsPerRow: number): ProjectTileRow[] {
    const tileRows: ProjectTileRow[] = [];
    for (const row of rows) {
        const last = tileRows[tileRows.length - 1];
        if (row.kind === 'heading') {
            tileRows.push(row);
        } else if (last?.kind === 'tiles' && last.items.length < columnsPerRow) {
            last.items.push(row);
        } else {
            tileRows.push({kind: 'tiles', items: [row]});
        }
    }
    return tileRows;
}
