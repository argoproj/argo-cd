import {queryOptions} from '@tanstack/react-query';

import {services} from './services';

// Keys are hierarchical so invalidating a prefix (e.g. projectQueries.byName) refreshes everything under it.
// Per-item keys sit under 'detail' so an item named "list" can't share a prefix with the list query.
export const projectQueries = {
    list: () => queryOptions({queryKey: ['projects', 'list'], queryFn: () => services.projects.list()}),
    byName: (name: string) => ['projects', 'detail', name] as const,
    detailed: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'detailed'], queryFn: () => services.projects.getDetailed(name)}),
    links: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'links'], queryFn: () => services.projects.getLinks(name)}),
    syncWindows: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'syncWindows'], queryFn: () => services.projects.getSyncWindows(name)}),
    events: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'events'], queryFn: () => services.projects.events(name)})
};

export const applicationQueries = {
    list: (projects: string[], objectListKind: string) =>
        queryOptions({queryKey: ['applications', 'list', projects, objectListKind], queryFn: () => services.applications.list(projects, objectListKind)})
};

export const repoQueries = {
    list: () => queryOptions({queryKey: ['repos', 'list'], queryFn: () => services.repos.list()})
};

export const clusterQueries = {
    list: () => queryOptions({queryKey: ['clusters', 'list'], queryFn: () => services.clusters.list()}),
    detail: (server: string) => queryOptions({queryKey: ['clusters', 'detail', server], queryFn: () => services.clusters.get(server, '')})
};

export const gpgkeyQueries = {
    list: () => queryOptions({queryKey: ['gpgkeys', 'list'], queryFn: () => services.gpgkeys.list()})
};
