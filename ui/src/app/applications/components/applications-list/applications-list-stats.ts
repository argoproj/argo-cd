import {Application, ApplicationDestination, SyncPolicy} from '../../../shared/models';
import {getAppAllSources, getOperationStateTitle} from '../utils';
import type {FilteredApp, FilterResult} from './applications-filter';

// The applications list views (filter sidebar, status bar, summary) render from these precomputed
// shapes rather than from the application array, so they don't depend on where the numbers come from.

export type StatusCounts = {[status: string]: number};

/** The part of a destination that identifies its cluster. */
export type DestinationCluster = Pick<ApplicationDestination, 'name' | 'server'>;

export interface MetadataValues {
    key: string;
    values: string[];
}

/** Flat counts over the applications currently shown, i.e. after filters and search. */
export interface AppsListStats {
    total: number;
    totalBySyncStatus: StatusCounts;
    totalByHealthStatus: StatusCounts;
    /** Keyed by the current hydrate operation phase, or 'None' when there is none. */
    totalByHydrationStatus: StatusCounts;
    /** Distinct name/server pairs. */
    destinations: DestinationCluster[];
    /** Distinct destination namespaces, including an unset one. */
    namespaces: string[];
}

/**
 * What the filter sidebar offers. Each `totalBy*` group counts the applications that pass every filter
 * except that group's own, so an option's count doesn't drop to zero as soon as a sibling is selected.
 * The option lists come from every loaded application, regardless of the filters.
 */
export interface AppsListFacets {
    totalBySyncStatus: StatusCounts;
    totalByHealthStatus: StatusCounts;
    /** Keyed by getHydrationStatus. */
    totalByHydrationStatus: StatusCounts;
    /** Keyed by getOperationStateTitle. */
    totalByOperationStatus: StatusCounts;
    /** Keyed by getAutoSyncStatus. */
    totalByAutoSyncStatus: StatusCounts;
    /** Distinct name/server pairs. */
    destinations: DestinationCluster[];
    namespaces: string[];
    labels: MetadataValues[];
    annotations: MetadataValues[];
    revisions: string[];
    repos: string[];
}

export function getAutoSyncStatus(syncPolicy?: SyncPolicy) {
    if (!syncPolicy || !syncPolicy.automated || syncPolicy.automated.enabled === false) {
        return 'Disabled';
    }
    return 'Enabled';
}

export function getHydrationStatus(app: Application) {
    if (!app.spec.sourceHydrator) {
        return 'None';
    }
    if (!app.status.sourceHydrator?.currentOperation) {
        return 'None';
    }
    return app.status.sourceHydrator.currentOperation.phase;
}

function increment(counts: StatusCounts, key: string) {
    counts[key] = (counts[key] || 0) + 1;
}

export function computeAppsListStats(applications: Application[]): AppsListStats {
    const stats: AppsListStats = {
        total: applications.length,
        totalBySyncStatus: {},
        totalByHealthStatus: {},
        totalByHydrationStatus: {},
        destinations: [],
        namespaces: []
    };
    const destinations = new Map<string, DestinationCluster>();
    const namespaces = new Set<string>();
    applications.forEach(app => {
        increment(stats.totalBySyncStatus, app.status.sync.status);
        increment(stats.totalByHealthStatus, app.status.health.status);
        increment(stats.totalByHydrationStatus, app.status.sourceHydrator?.currentOperation?.phase || 'None');
        destinations.set(`${app.spec.destination.name || ''}\n${app.spec.destination.server || ''}`, {name: app.spec.destination.name, server: app.spec.destination.server});
        namespaces.add(app.spec.destination.namespace);
    });
    stats.destinations = Array.from(destinations.values());
    stats.namespaces = Array.from(namespaces);
    return stats;
}

/** Collects the distinct values of each label or annotation key, in the order they are first seen. */
export function collectMetadataValues(items: Array<{metadata?: {[field: string]: any}}>, field: 'labels' | 'annotations'): MetadataValues[] {
    const byKey = new Map<string, Set<string>>();
    items.forEach(item => {
        const metadata: {[key: string]: string} = item.metadata?.[field];
        if (!metadata) {
            return;
        }
        Object.keys(metadata).forEach(key => {
            let values = byKey.get(key);
            if (!values) {
                values = new Set<string>();
                byKey.set(key, values);
            }
            values.add(metadata[key]);
        });
    });
    return Array.from(byKey.entries()).map(([key, values]) => ({key, values: Array.from(values)}));
}

function distinct(values: string[]): string[] {
    return Array.from(new Set(values.filter(value => !!value)));
}

export function computeAppsListFacets(apps: FilteredApp[]): AppsListFacets {
    const facets: AppsListFacets = {
        totalBySyncStatus: {},
        totalByHealthStatus: {},
        totalByHydrationStatus: {},
        totalByOperationStatus: {},
        totalByAutoSyncStatus: {},
        destinations: [],
        namespaces: [],
        labels: collectMetadataValues(apps, 'labels'),
        annotations: collectMetadataValues(apps, 'annotations'),
        revisions: [],
        repos: []
    };
    const destinations = new Map<string, DestinationCluster>();
    const namespaces: string[] = [];
    const revisions: string[] = [];
    const repos: string[] = [];

    apps.forEach(app => {
        destinations.set(`${app.spec.destination.name || ''}\n${app.spec.destination.server || ''}`, {name: app.spec.destination.name, server: app.spec.destination.server});
        namespaces.push(app.spec.destination.namespace);
        getAppAllSources(app).forEach(source => {
            revisions.push(source.targetRevision);
            repos.push(source.repoURL);
        });

        // An app counts towards a group only if every other filter matches it, so at most one filter may fail
        // and it has to be the group's own.
        const failing = (Object.keys(app.filterResult) as (keyof FilterResult)[]).filter(key => !app.filterResult[key]);
        if (failing.length > 1) {
            return;
        }
        const count = (filterType: keyof FilterResult, counts: StatusCounts, value: string) => {
            if (value && (failing.length === 0 || failing[0] === filterType)) {
                increment(counts, value);
            }
        };
        count('sync', facets.totalBySyncStatus, app.status.sync.status);
        count('health', facets.totalByHealthStatus, app.status.health.status);
        count('hydration', facets.totalByHydrationStatus, getHydrationStatus(app));
        count('operation', facets.totalByOperationStatus, getOperationStateTitle(app));
        count('autosync', facets.totalByAutoSyncStatus, getAutoSyncStatus(app.spec.syncPolicy));
    });

    facets.destinations = Array.from(destinations.values());
    facets.namespaces = distinct(namespaces);
    facets.revisions = distinct(revisions);
    facets.repos = distinct(repos);
    return facets;
}
