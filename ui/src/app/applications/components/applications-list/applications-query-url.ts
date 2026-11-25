// Query links are complete: older sidebar parameters cannot narrow the result invisibly.
export function applicationQueryParams(query: string): Record<string, string | null> {
    return {
        q: query,
        page: '0',
        search: null,
        proj: null,
        sync: null,
        statusMode: null,
        health: null,
        operation: null,
        hydration: null,
        autoSync: null,
        namespace: null,
        appNamespace: null,
        targetRevision: null,
        repo: null,
        cluster: null,
        labels: null,
        annotations: null,
        showFavorites: null
    };
}

export function applicationQueryLink(href: string, query: string): string {
    const url = new URL(href);
    Object.entries(applicationQueryParams(query)).forEach(([key, value]) => {
        if (value === null) url.searchParams.delete(key);
        else url.searchParams.set(key, value);
    });
    ['new', 'syncApp', 'syncAppNamespace', 'syncApps', 'refreshApps'].forEach(key => url.searchParams.delete(key));
    return url.toString();
}
