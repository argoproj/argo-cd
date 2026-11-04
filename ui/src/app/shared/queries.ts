import {queryOptions} from '@tanstack/react-query';

import {services} from './services';

// Keys are hierarchical so invalidating a prefix (e.g. projectQueries.byName) refreshes everything under it.
export const projectQueries = {
    byName: (name: string) => ['projects', name] as const,
    detailed: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'detailed'], queryFn: () => services.projects.getDetailed(name)}),
    links: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'links'], queryFn: () => services.projects.getLinks(name)}),
    syncWindows: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'syncWindows'], queryFn: () => services.projects.getSyncWindows(name)})
};

export const applicationQueries = {
    list: (projects: string[], objectListKind: string) =>
        queryOptions({queryKey: ['applications', 'list', projects, objectListKind], queryFn: () => services.applications.list(projects, objectListKind)})
};

export const repoQueries = {
    list: () => queryOptions({queryKey: ['repos', 'list'], queryFn: () => services.repos.list()})
};

export const clusterQueries = {
    list: () => queryOptions({queryKey: ['clusters', 'list'], queryFn: () => services.clusters.list()})
};

export const gpgkeyQueries = {
    list: () => queryOptions({queryKey: ['gpgkeys', 'list'], queryFn: () => services.gpgkeys.list()})
};
