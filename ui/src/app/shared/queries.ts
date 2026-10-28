import {queryOptions} from '@tanstack/react-query';

import {services} from './services';

// Keys are hierarchical so invalidating a prefix (e.g. projectQueries.byName) refreshes everything under it.
// Per-item keys sit under 'detail' so an item named "list" can't share a prefix with the list query.
export const projectQueries = {
    list: () => queryOptions({queryKey: ['projects', 'list'], queryFn: () => services.projects.list()}),
    names: () =>
        queryOptions({
            queryKey: ['projects', 'list', 'names'],
            queryFn: () => services.projects.list('items.metadata.name').then(projects => projects.map(proj => proj.metadata.name).sort())
        }),
    byName: (name: string) => ['projects', 'detail', name] as const,
    detailed: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'detailed'], queryFn: () => services.projects.getDetailed(name)}),
    links: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'links'], queryFn: () => services.projects.getLinks(name)}),
    syncWindows: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'syncWindows'], queryFn: () => services.projects.getSyncWindows(name)}),
    events: (name: string) => queryOptions({queryKey: [...projectQueries.byName(name), 'events'], queryFn: () => services.projects.events(name)})
};

export const applicationQueries = {
    list: (projects: string[], objectListKind: string) =>
        queryOptions({queryKey: ['applications', 'list', projects, objectListKind], queryFn: () => services.applications.list(projects, objectListKind)}),
    names: (projects: string[], objectListKind: string) =>
        queryOptions({
            queryKey: ['applications', 'list', projects, objectListKind, 'names'],
            queryFn: () => services.applications.list(projects, objectListKind, {fields: ['items.metadata.name']}).then(list => list.items)
        })
};

export const repoQueries = {
    all: ['repos'] as const,
    list: () => queryOptions({queryKey: ['repos', 'list'], queryFn: () => services.repos.list()}),
    // Repositories and credential templates, read and (with the hydrator) write, as the repositories page lists them.
    connections: (hydratorEnabled: boolean) =>
        queryOptions({
            queryKey: ['repos', 'connections', hydratorEnabled],
            queryFn: () =>
                Promise.all([
                    services.repos.list(),
                    services.repocreds.list(),
                    hydratorEnabled ? services.repos.listWrite() : Promise.resolve([]),
                    hydratorEnabled ? services.repocreds.listWrite() : Promise.resolve([])
                ])
        })
};

export const clusterQueries = {
    list: () => queryOptions({queryKey: ['clusters', 'list'], queryFn: () => services.clusters.list()}),
    detail: (server: string) => queryOptions({queryKey: ['clusters', 'detail', server], queryFn: () => services.clusters.get(server, '')})
};

export const certQueries = {
    list: () => queryOptions({queryKey: ['certs', 'list'], queryFn: () => services.certs.list()})
};

export const gpgkeyQueries = {
    list: () => queryOptions({queryKey: ['gpgkeys', 'list'], queryFn: () => services.gpgkeys.list()})
};

export const accountQueries = {
    list: () => queryOptions({queryKey: ['accounts', 'list'], queryFn: () => services.accounts.list()}),
    detail: (name: string) => queryOptions({queryKey: ['accounts', 'detail', name], queryFn: () => services.accounts.get(name)})
};
