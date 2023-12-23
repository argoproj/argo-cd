import * as deepMerge from 'deepmerge';
import {Observable} from 'rxjs';
import {map, repeat, retry} from 'rxjs/operators';

import * as models from '../models';
import {isValidURL} from '../utils';
import requests from './requests';
import {getRootPathByApp, getRootPathByPath, isApp, isInvokedFromAppsPath} from '../../applications/components/utils';
import {ContextApis} from '../context';
import {History} from 'history';

interface QueryOptions {
    fields: string[];
    exclude?: boolean;
    selector?: string;
    appNamespace?: string;
}

function optionsToSearch(options?: QueryOptions) {
    if (options) {
        return {fields: (options.exclude ? '-' : '') + options.fields.join(','), selector: options.selector || '', appNamespace: options.appNamespace || ''};
    }
    return {};
}

function getQuery(projects: string[], isFromApps: boolean, options?: QueryOptions): any {
    if (isFromApps) {
        return {projects, ...optionsToSearch(options)};
    } else {
        return {...optionsToSearch(options)};
    }
}

export class ApplicationsService {
    public list(
        projects: string[],
        ctx: ContextApis & {
            history: History<unknown>;
        },
        options?: QueryOptions
    ): Promise<models.AbstractApplicationList> {
        const isFromApps = isInvokedFromAppsPath(ctx.history.location.pathname);
        return requests
            .get(getRootPathByPath(ctx.history.location.pathname))
            .query(getQuery(projects, isFromApps, options))
            .then(res => {
                if (isFromApps) {
                    return res.body as models.ApplicationList;
                } else {
                    return res.body as models.ApplicationSetList;
                }
            })
            .then(list => {
                list.items = (list.items || []).map(app => this.parseAppFields(app, isFromApps));

                return list;
            });
    }

    public get(name: string, appNamespace: string, pathname: string, refresh?: 'normal' | 'hard'): Promise<models.AbstractApplication> {
        const query: {[key: string]: string} = {};
        const isFromApps = isInvokedFromAppsPath(pathname);
        if (refresh) {
            query.refresh = refresh;
        }
        if (appNamespace) {
            query.appNamespace = appNamespace;
        }

        return requests
            .get(`${getRootPathByPath(pathname)}/${name}`)
            .query(query)
            .then(res => this.parseAppFields(res.body, isFromApps));
    }

    public getApplicationSyncWindowState(name: string, appNamespace: string): Promise<models.ApplicationSyncWindowState> {
        return requests
            .get(`/applications/${name}/syncwindows`)
            .query({name, appNamespace})
            .then(res => res.body as models.ApplicationSyncWindowState);
    }

    public revisionMetadata(name: string, appNamespace: string, revision: string): Promise<models.RevisionMetadata> {
        return requests
            .get(`/applications/${name}/revisions/${revision || 'HEAD'}/metadata`)
            .query({appNamespace})
            .then(res => res.body as models.RevisionMetadata);
    }

    public revisionChartDetails(name: string, appNamespace: string, revision: string): Promise<models.ChartDetails> {
        return requests
            .get(`/applications/${name}/revisions/${revision || 'HEAD'}/chartdetails`)
            .query({appNamespace})
            .then(res => res.body as models.ChartDetails);
    }

    public resourceTree(name: string, appNamespace: string, pathname: string): Promise<models.AbstractApplicationTree> {
        return requests
            .get(`${getRootPathByPath(pathname)}/${name}/resource-tree`)
            .query({appNamespace})

            .then(res => res.body as models.AbstractApplicationTree);
    }

    public watchResourceTree(name: string, appNamespace: string, pathname: string): Observable<models.ApplicationTree> {
        return requests
            .loadEventSource(`/stream${getRootPathByPath(pathname)}/${name}/resource-tree?appNamespace=${appNamespace}`)
            .pipe(map(data => JSON.parse(data).result as models.ApplicationTree));
    }

    public managedResources(name: string, appNamespace: string, pathname: string, options: {id?: models.ResourceID; fields?: string[]} = {}): Promise<models.ResourceDiff[]> {
        return requests
            .get(`${getRootPathByPath(pathname)}/${name}/managed-resources`)
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

    public getManifest(name: string, appNamespace: string, pathname: string, revision: string): Promise<models.ManifestResponse> {
        return requests
            .get(`${getRootPathByPath(pathname)}/${name}/manifests`)
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

    public update(app: models.AbstractApplication, query: {validate?: boolean} = {}): Promise<models.AbstractApplication> {
        const isAnApp = isApp(app);
        return requests
            .put(`${getRootPathByApp(app)}/${app.metadata.name}`)
            .query(query)
            .send(isAnApp ? (app as models.Application) : (app as models.ApplicationSet))
            .then(res => this.parseAppFields(res.body, isAnApp));
    }

    public create(app: models.AbstractApplication): Promise<models.AbstractApplication> {
        const isAnApp = isApp(app);
        // Namespace may be specified in the app name. We need to parse and
        // handle it accordingly.
        if (app.metadata.name.includes('/')) {
            const nns = app.metadata.name.split('/', 2);
            app.metadata.name = nns[1];
            app.metadata.namespace = nns[0];
        }
        return requests
            .post(getRootPathByApp(app))
            .send(isAnApp ? (app as models.Application) : (app as models.ApplicationSet))
            .then(res => this.parseAppFields(res.body, isAnApp));
    }

    public delete(name: string, appNamespace: string, propagationPolicy: string): Promise<boolean> {
        let cascade = true;
        if (propagationPolicy === 'non-cascading') {
            propagationPolicy = '';
            cascade = false;
        }
        // let isFromApps = isInvokedFromApps();
        return requests
            .delete(`/applications//${name}`)
            .query({
                cascade,
                propagationPolicy,
                appNamespace
            })
            .send({})
            .then(() => true);
    }

    public watch(
        pathname: string,
        query?: {name?: string; resourceVersion?: string; projects?: string[]; appNamespace?: string},
        options?: QueryOptions
    ): Observable<models.ApplicationWatchEvent> {
        const search = new URLSearchParams();
        const isFromApps = isInvokedFromAppsPath(pathname);
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
            search.set('fields', searchOptions.fields);
            search.set('selector', searchOptions.selector);
            search.set('appNamespace', searchOptions.appNamespace);
            if (isFromApps) {
                query?.projects?.forEach(project => search.append('projects', project));
            }
        }
        const searchStr = search.toString();
        const url = `/stream${getRootPathByPath(pathname)}${(searchStr && '?' + searchStr) || ''}`;
        return requests
            .loadEventSource(url)
            .pipe(repeat())
            .pipe(retry())
            .pipe(map(data => JSON.parse(data).result as models.ApplicationWatchEvent))
            .pipe(
                map(watchEvent => {
                    watchEvent.application = this.parseAppFields(watchEvent.application, isFromApps);
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

    public rollback(name: string, appNamespace: string, id: number): Promise<boolean> {
        return requests
            .post(`/applications/${name}/rollback`)
            .send({id, appNamespace})
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

    public getResource(name: string, appNamespace: string, pathname: string, resource: models.ResourceNode): Promise<models.State> {
        return requests
            .get(`${getRootPathByPath(pathname)}/${name}/resource`)
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

    public runResourceAction(name: string, appNamespace: string, resource: models.ResourceNode, action: string): Promise<models.ResourceAction[]> {
        return requests
            .post(`/applications/${name}/resource/actions`)
            .query({
                appNamespace,
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group
            })
            .send(JSON.stringify(action))
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
        // let isFromApps = isInvokedFromApps();
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
        // let isFromApps = isInvokedFromApps();
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
        previous?: boolean;
    }): URLSearchParams {
        const {appNamespace, containerName, namespace, podName, resource, tail, sinceSeconds, untilTime, filter, previous} = query;
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
        if (sinceSeconds) {
            search.set('sinceSeconds', sinceSeconds.toString());
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
        // The API requires that this field be set to a non-empty string.
        search.set('sinceSeconds', '0');
        return search;
    }

    private parseAppFields(data: any, isFromApps: boolean): models.AbstractApplication {
        if (isFromApps) {
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
        } else {
            data = deepMerge(
                {
                    apiVersion: 'argoproj.io/v1alpha1',
                    kind: 'ApplicationSet',
                    status: {
                        resources: []
                    }
                },
                data
            );
            (data as models.ApplicationSet).status.resources[0].kind = 'Application';
            (data as models.ApplicationSet).status.resources[0].group = 'argoproj.io';
            return data as models.ApplicationSet;
        }
    }
}
