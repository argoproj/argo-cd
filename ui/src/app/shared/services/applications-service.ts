import * as deepMerge from 'deepmerge';
import {Observable} from 'rxjs';
import {map, repeat, retry} from 'rxjs/operators';

import * as models from '../models';
import {isValidURL} from '../utils';
import requests from './requests';
import {AppsListPreferences} from './view-preferences-service';

export interface QueryOptions {
    fields: string[];
    exclude?: boolean;
    selector?: string;
    appNamespace?: string;

    favorites?: string[];
    clusters?: string[];
    namespaces?: string[];
    labels?: string[];
    syncStatuses?: string[];
    healthStatuses?: string[];
    autoSyncEnabled?: boolean;
    search?: string;
    sortBy?: 'name' | 'createdAt' | 'synchronizedAt';
    minName?: string;
    maxName?: string;
    minCreatedAt?: string;
    maxCreatedAt?: string;
    minSynchronizedAt?: string;
    maxSynchronizedAt?: string;
    offset?: number;
    limit?: number;
}

function optionsToSearch(options?: QueryOptions) {
    if (!options) {
        return {};
    }

    const search: any = {
        fields: (options.exclude ? '-' : '') + options.fields.join(','),
        selector: options.selector || '',
        appNamespace: options.appNamespace || ''
    };

    // Add new server-side filter params
    if (options.clusters && options.clusters.length > 0) {
        search.clusters = options.clusters;
    }
    if (options.namespaces && options.namespaces.length > 0) {
        search.namespaces = options.namespaces;
    }
    if (options.labels && options.labels.length > 0) {
        search.labels = options.labels;
    }
    if (options.syncStatuses && options.syncStatuses.length > 0) {
        search.syncStatuses = options.syncStatuses;
    }
    if (options.healthStatuses && options.healthStatuses.length > 0) {
        search.healthStatuses = options.healthStatuses;
    }
    if (options.autoSyncEnabled !== undefined) {
        search.autoSyncs = options.autoSyncEnabled ? 'Enabled' : 'Disabled';
    }
    if (options.favorites && options.favorites.length > 0) {
        search.names = options.favorites;
    }
    if (options.search) {
        search.search = options.search;
    }
    if (options.sortBy) {
        search.sortBy = options.sortBy;
    }
    if (options.minName) {
        search.minName = options.minName;
    }
    if (options.maxName) {
        search.maxName = options.maxName;
    }
    if (options.minCreatedAt) {
        search.minCreatedAt = options.minCreatedAt;
    }
    if (options.maxCreatedAt) {
        search.maxCreatedAt = options.maxCreatedAt;
    }
    if (options.minSynchronizedAt) {
        search.minSynchronizedAt = options.minSynchronizedAt;
    }
    if (options.maxSynchronizedAt) {
        search.maxSynchronizedAt = options.maxSynchronizedAt;
    }
    if (options.offset !== undefined) {
        search.offset = options.offset;
    }
    if (options.limit !== undefined) {
        search.limit = options.limit;
    }

    return search;
}

function filtersToQueryOptions(pref: AppsListPreferences, search: string): Partial<QueryOptions> {
    const options: Partial<QueryOptions> = {};

    if (pref.clustersFilter && pref.clustersFilter.length > 0) {
        // Extract server URLs from cluster filter format
        options.clusters = pref.clustersFilter.map(filterString => {
            const match = filterString.match('^(.*) [(](http.*)[)]$');
            if (match?.length === 3) {
                return match[2]; // Return URL
            }
            return filterString;
        });
    }
    if (pref.namespacesFilter && pref.namespacesFilter.length > 0) {
        options.namespaces = pref.namespacesFilter;
    }
    if (pref.labelsFilter && pref.labelsFilter.length > 0) {
        options.labels = pref.labelsFilter;
    }
    if (pref.syncFilter && pref.syncFilter.length > 0) {
        options.syncStatuses = pref.syncFilter;
    }
    if (pref.healthFilter && pref.healthFilter.length > 0) {
        options.healthStatuses = pref.healthFilter;
    }
    if (pref.autoSyncFilter && pref.autoSyncFilter.length > 0) {
        // Convert "Enabled"/"Disabled" to boolean
        if (pref.autoSyncFilter.includes('Enabled') && !pref.autoSyncFilter.includes('Disabled')) {
            options.autoSyncEnabled = true;
        } else if (pref.autoSyncFilter.includes('Disabled') && !pref.autoSyncFilter.includes('Enabled')) {
            options.autoSyncEnabled = false;
        }
    }

    if (pref.showFavorites && pref.favoritesAppList && pref.favoritesAppList.length > 0) {
        options.favorites = pref.favoritesAppList;
    }

    if (search && search.trim() !== '') {
        options.search = search;
    }

    return options;
}

export class ApplicationsService {
    constructor() {}

    public filtersToQueryOptions = filtersToQueryOptions;

    public list(projects: string[], options?: QueryOptions): Promise<models.ApplicationList> {
        return requests
            .get('/applications')
            .query({projects, ...optionsToSearch(options)})
            .then(res => res.body as models.ApplicationList)
            .then(list => {
                list.items = (list.items || []).map(app => this.parseAppFields(app));
                return list;
            });
    }

    public get(name: string, appNamespace: string, refresh?: 'normal' | 'hard'): Promise<models.Application> {
        const query: {[key: string]: string} = {};
        if (refresh) {
            query.refresh = refresh;
        }
        if (appNamespace) {
            query.appNamespace = appNamespace;
        }
        return requests
            .get(`/applications/${name}`)
            .query(query)
            .then(res => this.parseAppFields(res.body));
    }

    public getApplicationSyncWindowState(name: string, appNamespace: string): Promise<models.ApplicationSyncWindowState> {
        return requests
            .get(`/applications/${name}/syncwindows`)
            .query({name, appNamespace})
            .then(res => res.body as models.ApplicationSyncWindowState);
    }

    public ociMetadata(name: string, appNamespace: string, revision: string, sourceIndex: number, versionId: number): Promise<models.OCIMetadata> {
        let r = requests.get(`/applications/${name}/revisions/${revision || 'HEAD'}/ocimetadata`).query({appNamespace});
        if (sourceIndex !== null) {
            r = r.query({sourceIndex});
        }
        if (versionId !== null) {
            r = r.query({versionId});
        }
        return r.then(res => res.body as models.OCIMetadata);
    }

    public revisionMetadata(name: string, appNamespace: string, revision: string, sourceIndex: number | null, versionId: number | null): Promise<models.RevisionMetadata> {
        let r = requests.get(`/applications/${name}/revisions/${revision || 'HEAD'}/metadata`).query({appNamespace});
        if (sourceIndex !== null) {
            r = r.query({sourceIndex});
        }
        if (versionId !== null) {
            r = r.query({versionId});
        }
        return r.then(res => res.body as models.RevisionMetadata);
    }

    public revisionChartDetails(name: string, appNamespace: string, revision: string, sourceIndex: number, versionId: number | null): Promise<models.ChartDetails> {
        let r = requests.get(`/applications/${name}/revisions/${revision || 'HEAD'}/chartdetails`).query({appNamespace});
        if (sourceIndex !== null) {
            r = r.query({sourceIndex});
        }
        if (versionId !== null) {
            r = r.query({versionId});
        }
        return r.then(res => res.body as models.ChartDetails);
    }

    public resourceTree(name: string, appNamespace: string): Promise<models.ApplicationTree> {
        return requests
            .get(`/applications/${name}/resource-tree`)
            .query({appNamespace})
            .then(res => res.body as models.ApplicationTree);
    }

    public watchResourceTree(name: string, appNamespace: string): Observable<models.ApplicationTree> {
        return requests
            .loadEventSource(`/stream/applications/${name}/resource-tree?appNamespace=${appNamespace}`)
            .pipe(map(data => JSON.parse(data).result as models.ApplicationTree));
    }

    public managedResources(name: string, appNamespace: string, options: {id?: models.ResourceID; fields?: string[]} = {}): Promise<models.ResourceDiff[]> {
        return requests
            .get(`/applications/${name}/managed-resources`)
            .query(`appNamespace=${appNamespace.toString()}`)
            .query({...options.id, fields: (options.fields || []).join(',')})
            .then(res => (res.body.items as any[]) || [])
            .then(items => {
                items.forEach(item => {
                    if (item.liveState) {
                        item.liveState = JSON.parse(item.liveState);
                    }
                    if (item.targetState) {
                        item.targetState = JSON.parse(item.targetState);
                    }
                    if (item.predictedLiveState) {
                        item.predictedLiveState = JSON.parse(item.predictedLiveState);
                    }
                    if (item.normalizedLiveState) {
                        item.normalizedLiveState = JSON.parse(item.normalizedLiveState);
                    }
                });
                return items as models.ResourceDiff[];
            });
    }

    public getManifest(name: string, appNamespace: string, revision: string): Promise<models.ManifestResponse> {
        return requests
            .get(`/applications/${name}/manifests`)
            .query({name, revision, appNamespace})
            .then(res => res.body as models.ManifestResponse);
    }

    public updateSpec(appName: string, appNamespace: string, spec: models.ApplicationSpec): Promise<models.ApplicationSpec> {
        return requests
            .put(`/applications/${appName}/spec`)
            .query({appNamespace})
            .send(spec)
            .then(res => res.body as models.ApplicationSpec);
    }

    public update(app: models.Application, query: {validate?: boolean} = {}): Promise<models.Application> {
        return requests
            .put(`/applications/${app.metadata.name}`)
            .query(query)
            .send(app)
            .then(res => this.parseAppFields(res.body));
    }

    public create(app: models.Application): Promise<models.Application> {
        // Namespace may be specified in the app name. We need to parse and
        // handle it accordingly.
        if (app.metadata.name.includes('/')) {
            const nns = app.metadata.name.split('/', 2);
            app.metadata.name = nns[1];
            app.metadata.namespace = nns[0];
        }
        return requests
            .post(`/applications`)
            .send(app)
            .then(res => this.parseAppFields(res.body));
    }

    public delete(name: string, appNamespace: string, propagationPolicy: string): Promise<boolean> {
        let cascade = true;
        if (propagationPolicy === 'non-cascading') {
            propagationPolicy = '';
            cascade = false;
        }
        return requests
            .delete(`/applications/${name}`)
            .query({
                cascade,
                propagationPolicy,
                appNamespace
            })
            .send({})
            .then(() => true);
    }

    public watch(query?: {name?: string; resourceVersion?: string; projects?: string[]; appNamespace?: string}, options?: QueryOptions): Observable<models.ApplicationWatchEvent> {
        const search = new URLSearchParams();
        if (query) {
            if (query.name) {
                search.set('name', query.name);
            }
            if (query.resourceVersion) {
                search.set('resourceVersion', query.resourceVersion);
            }
            if (query.appNamespace) {
                search.set('appNamespace', query.appNamespace);
            }
        }
        if (options) {
            const searchOptions = optionsToSearch(options);
            Object.entries(searchOptions).forEach(([key, value]) => {
                search.set(key, String(value));
            });
            query?.projects?.forEach(project => search.append('projects', project));
        }
        const searchStr = search.toString();
        const url = `/stream/applications${(searchStr && '?' + searchStr) || ''}`;
        return requests
            .loadEventSource(url)
            .pipe(repeat())
            .pipe(retry())
            .pipe(map(data => JSON.parse(data).result as models.ApplicationWatchEvent))
            .pipe(
                map(watchEvent => {
                    watchEvent.application = this.parseAppFields(watchEvent.application);
                    return watchEvent;
                })
            );
    }

    public sync(
        name: string,
        appNamespace: string,
        revision: string,
        prune: boolean,
        dryRun: boolean,
        strategy: models.SyncStrategy,
        resources: models.SyncOperationResource[],
        syncOptions?: string[],
        retryStrategy?: models.RetryStrategy
    ): Promise<boolean> {
        return requests
            .post(`/applications/${name}/sync`)
            .send({
                appNamespace,
                revision,
                prune: !!prune,
                dryRun: !!dryRun,
                strategy,
                resources,
                syncOptions: syncOptions ? {items: syncOptions} : null,
                retryStrategy
            })
            .then(() => true);
    }

    public rollback(name: string, appNamespace: string, id: number, prune?: boolean): Promise<boolean> {
        return requests
            .post(`/applications/${name}/rollback`)
            .send({id, appNamespace, prune})
            .then(() => true);
    }

    public getDownloadLogsURL(
        applicationName: string,
        appNamespace: string,
        namespace: string,
        podName: string,
        resource: {group: string; kind: string; name: string},
        containerName: string
    ): string {
        const search = this.getLogsQuery({namespace, appNamespace, podName, resource, containerName, follow: false});
        search.set('download', 'true');
        return `api/v1/applications/${applicationName}/logs?${search.toString()}`;
    }

    public getContainerLogs(query: {
        applicationName: string;
        appNamespace: string;
        namespace: string;
        podName: string;
        resource: {group: string; kind: string; name: string};
        containerName: string;
        tail?: number;
        follow?: boolean;
        sinceSeconds?: number;
        untilTime?: string;
        filter?: string;
        matchCase?: boolean;
        previous?: boolean;
    }): Observable<models.LogEntry> {
        const {applicationName} = query;
        const search = this.getLogsQuery(query);
        const entries = requests.loadEventSource(`/applications/${applicationName}/logs?${search.toString()}`).pipe(map(data => JSON.parse(data).result as models.LogEntry));
        let first = true;
        return new Observable(observer => {
            const subscription = entries.subscribe(
                entry => {
                    if (entry.last) {
                        first = true;
                        observer.complete();
                        subscription.unsubscribe();
                    } else {
                        observer.next({...entry, first});
                        first = false;
                    }
                },
                err => {
                    first = true;
                    observer.error(err);
                },
                () => {
                    first = true;
                    observer.complete();
                }
            );
            return () => subscription.unsubscribe();
        });
    }

    public getResource(name: string, appNamespace: string, resource: models.ResourceNode): Promise<models.State> {
        return requests
            .get(`/applications/${name}/resource`)
            .query({
                name: resource.name,
                appNamespace,
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group || '' // The group query param must be present even if empty.
            })
            .then(res => res.body as {manifest: string})
            .then(res => JSON.parse(res.manifest) as models.State);
    }

    public getResourceActions(name: string, appNamespace: string, resource: models.ResourceNode): Promise<models.ResourceAction[]> {
        return requests
            .get(`/applications/${name}/resource/actions`)
            .query({
                appNamespace,
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group
            })
            .then(res => {
                const actions = (res.body.actions as models.ResourceAction[]) || [];
                actions.sort((actionA, actionB) => actionA.name.localeCompare(actionB.name));
                return actions;
            });
    }

    public runResourceAction(
        name: string,
        appNamespace: string,
        resource: models.ResourceNode,
        action: string,
        resourceActionParameters: models.ResourceActionParam[]
    ): Promise<models.ResourceAction[]> {
        return requests
            .post(`/applications/${name}/resource/actions/v2`)
            .send(
                JSON.stringify({
                    appNamespace,
                    namespace: resource.namespace,
                    resourceName: resource.name,
                    version: resource.version,
                    kind: resource.kind,
                    group: resource.group,
                    resourceActionParameters: resourceActionParameters,
                    action
                })
            )
            .then(res => (res.body.actions as models.ResourceAction[]) || []);
    }

    public patchResource(name: string, appNamespace: string, resource: models.ResourceNode, patch: string, patchType: string): Promise<models.State> {
        return requests
            .post(`/applications/${name}/resource`)
            .query({
                name: resource.name,
                appNamespace,
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group || '', // The group query param must be present even if empty.
                patchType
            })
            .send(JSON.stringify(patch))
            .then(res => res.body as {manifest: string})
            .then(res => JSON.parse(res.manifest) as models.State);
    }

    public deleteResource(applicationName: string, appNamespace: string, resource: models.ResourceNode, force: boolean, orphan: boolean): Promise<any> {
        return requests
            .delete(`/applications/${applicationName}/resource`)
            .query({
                name: resource.name,
                appNamespace,
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group || '', // The group query param must be present even if empty.
                force,
                orphan
            })
            .send()
            .then(() => true);
    }

    public events(applicationName: string, appNamespace: string): Promise<models.Event[]> {
        return requests
            .get(`/applications/${applicationName}/events`)
            .query({appNamespace})
            .send()
            .then(res => (res.body as models.EventList).items || []);
    }

    public resourceEvents(
        applicationName: string,
        appNamespace: string,
        resource: {
            namespace: string;
            name: string;
            uid: string;
        }
    ): Promise<models.Event[]> {
        return requests
            .get(`/applications/${applicationName}/events`)
            .query({
                appNamespace,
                resourceUID: resource.uid,
                resourceNamespace: resource.namespace,
                resourceName: resource.name
            })
            .send()
            .then(res => (res.body as models.EventList).items || []);
    }

    public terminateOperation(applicationName: string, appNamespace: string): Promise<boolean> {
        return requests
            .delete(`/applications/${applicationName}/operation`)
            .query({appNamespace})
            .send()
            .then(() => true);
    }

    public getLinks(applicationName: string, namespace: string): Promise<models.LinksResponse> {
        return requests
            .get(`/applications/${applicationName}/links`)
            .query({namespace})
            .send()
            .then(res => res.body as models.LinksResponse);
    }

    public getResourceLinks(applicationName: string, appNamespace: string, resource: models.ResourceNode): Promise<models.LinksResponse> {
        return requests
            .get(`/applications/${applicationName}/resource/links`)
            .query({
                name: resource.name,
                appNamespace,
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group || '' // The group query param must be present even if empty.
            })
            .send()
            .then(res => {
                const links = res.body as models.LinksResponse;
                const items: models.LinkInfo[] = [];
                (links?.items || []).forEach(link => {
                    if (isValidURL(link.url)) {
                        items.push(link);
                    }
                });
                links.items = items;
                return links;
            });
    }

    private getLogsQuery(query: {
        namespace: string;
        appNamespace: string;
        podName: string;
        resource: {group: string; kind: string; name: string};
        containerName: string;
        tail?: number;
        follow?: boolean;
        sinceSeconds?: number;
        untilTime?: string;
        filter?: string;
        matchCase?: boolean;
        previous?: boolean;
    }): URLSearchParams {
        const {appNamespace, containerName, namespace, podName, resource, tail, sinceSeconds, untilTime, filter, previous, matchCase} = query;
        let {follow} = query;
        if (follow === undefined || follow === null) {
            follow = true;
        }
        const search = new URLSearchParams();
        search.set('appNamespace', appNamespace);
        search.set('container', containerName);
        search.set('namespace', namespace);
        search.set('follow', follow.toString());
        if (podName) {
            search.set('podName', podName);
        } else {
            search.set('group', resource.group);
            search.set('kind', resource.kind);
            search.set('resourceName', resource.name);
        }
        if (tail) {
            search.set('tailLines', tail.toString());
        }
        if (untilTime) {
            search.set('untilTime', untilTime);
        }
        if (filter) {
            search.set('filter', filter);
        }
        if (previous) {
            search.set('previous', previous.toString());
        }
        if (matchCase) {
            search.set('matchCase', matchCase.toString());
        }
        // The API requires that this field be set to a non-empty string.
        if (sinceSeconds) {
            search.set('sinceSeconds', sinceSeconds.toString());
        } else {
            search.set('sinceSeconds', '0');
        }
        return search;
    }

    private parseAppFields(data: any): models.Application {
        data = deepMerge(
            {
                apiVersion: 'argoproj.io/v1alpha1',
                kind: 'Application',
                spec: {
                    project: 'default'
                },
                status: {
                    resources: [],
                    summary: {}
                }
            },
            data
        );

        return data as models.Application;
    }

    public async getApplicationSet(name: string, namespace: string): Promise<models.ApplicationSet> {
        return requests
            .get(`/applicationsets/${name}`)
            .query({appsetNamespace: namespace})
            .then(res => res.body as models.ApplicationSet);
    }

    public async listApplicationSets(): Promise<models.ApplicationSetList> {
        return requests.get(`/applicationsets`).then(res => res.body as models.ApplicationSetList);
    }
}
