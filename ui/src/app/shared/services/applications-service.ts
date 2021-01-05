import * as deepMerge from 'deepmerge';
import {Observable} from 'rxjs';

import * as models from '../models';
import requests from './requests';

interface QueryOptions {
    fields: string[];
    exclude?: boolean;
    selector?: string;
}

function optionsToSearch(options?: QueryOptions) {
    if (options) {
        return {fields: (options.exclude ? '-' : '') + options.fields.join(','), selector: options.selector || ''};
    }
    return {};
}

export class ApplicationsService {
    public list(projects: string[], options?: QueryOptions): Promise<models.ApplicationList> {
        return requests
            .get('/applications')
            .query({project: projects, ...optionsToSearch(options)})
            .then(res => res.body as models.ApplicationList)
            .then(list => {
                list.items = (list.items || []).map(app => this.parseAppFields(app));
                return list;
            });
    }

    public get(name: string, refresh?: 'normal' | 'hard'): Promise<models.Application> {
        const query: {[key: string]: string} = {};
        if (refresh) {
            query.refresh = refresh;
        }
        return requests
            .get(`/applications/${name}`)
            .query(query)
            .then(res => this.parseAppFields(res.body));
    }

    public getApplicationSyncWindowState(name: string): Promise<models.ApplicationSyncWindowState> {
        return requests
            .get(`/applications/${name}/syncwindows`)
            .query({name})
            .then(res => res.body as models.ApplicationSyncWindowState);
    }

    public revisionMetadata(name: string, revision: string): Promise<models.RevisionMetadata> {
        return requests.get(`/applications/${name}/revisions/${revision || 'HEAD'}/metadata`).then(res => res.body as models.RevisionMetadata);
    }

    public resourceTree(name: string): Promise<models.ApplicationTree> {
        return requests.get(`/applications/${name}/resource-tree`).then(res => res.body as models.ApplicationTree);
    }

    public watchResourceTree(name: string): Observable<models.ApplicationTree> {
        return requests.loadEventSource(`/stream/applications/${name}/resource-tree`).map(data => JSON.parse(data).result as models.ApplicationTree);
    }

    public managedResources(name: string, options: {id?: models.ResourceID; fields?: string[]} = {}): Promise<models.ResourceDiff[]> {
        return requests
            .get(`/applications/${name}/managed-resources`)
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

    public getNodes(name: string): Promise<models.InfraNode[]> {
        return requests.get(`/applications/${name}/nodes`).then(res => {
            return (res.body.items as models.InfraNode[]) || [];
        });
    }

    public getManifest(name: string, revision: string): Promise<models.ManifestResponse> {
        return requests
            .get(`/applications/${name}/manifests`)
            .query({name, revision})
            .then(res => res.body as models.ManifestResponse);
    }

    public updateSpec(appName: string, spec: models.ApplicationSpec): Promise<models.ApplicationSpec> {
        return requests
            .put(`/applications/${appName}/spec`)
            .send(spec)
            .then(res => res.body as models.ApplicationSpec);
    }

    public update(app: models.Application): Promise<models.Application> {
        return requests
            .put(`/applications/${app.metadata.name}`)
            .send(app)
            .then(res => this.parseAppFields(res.body));
    }

    public create(app: models.Application): Promise<models.Application> {
        return requests
            .post(`/applications`)
            .send(app)
            .then(res => this.parseAppFields(res.body));
    }

    public delete(name: string, cascade: boolean): Promise<boolean> {
        return requests
            .delete(`/applications/${name}`)
            .query({cascade})
            .send({})
            .then(() => true);
    }

    public watch(query?: {name?: string; resourceVersion?: string}, options?: QueryOptions): Observable<models.ApplicationWatchEvent> {
        const search = new URLSearchParams();
        if (query) {
            if (query.name) {
                search.set('name', query.name);
            }
            if (query.resourceVersion) {
                search.set('resourceVersion', query.resourceVersion);
            }
        }
        if (options) {
            const searchOptions = optionsToSearch(options);
            search.set('fields', searchOptions.fields);
            search.set('selector', searchOptions.selector);
        }
        const searchStr = search.toString();
        const url = `/stream/applications${(searchStr && '?' + searchStr) || ''}`;
        return requests
            .loadEventSource(url)
            .repeat()
            .retry()
            .map(data => JSON.parse(data).result as models.ApplicationWatchEvent)
            .map(watchEvent => {
                watchEvent.application = this.parseAppFields(watchEvent.application);
                return watchEvent;
            });
    }

    public sync(name: string, revision: string, prune: boolean, dryRun: boolean, strategy: models.SyncStrategy, resources: models.SyncOperationResource[]): Promise<boolean> {
        return requests
            .post(`/applications/${name}/sync`)
            .send({revision, prune: !!prune, dryRun: !!dryRun, strategy, resources})
            .then(() => true);
    }

    public rollback(name: string, id: number): Promise<boolean> {
        return requests
            .post(`/applications/${name}/rollback`)
            .send({id})
            .then(() => true);
    }

    public getContainerLogs(applicationName: string, namespace: string, podName: string, containerName: string): Observable<models.LogEntry> {
        const entries = requests
            .loadEventSource(`/applications/${applicationName}/pods/${podName}/logs?container=${containerName}&follow=true&namespace=${namespace}`)
            .map(data => JSON.parse(data).result as models.LogEntry);
        return new Observable(observer => {
            const subscription = entries.subscribe(
                entry => {
                    if (entry.last) {
                        observer.complete();
                        subscription.unsubscribe();
                    } else {
                        observer.next(entry);
                    }
                },
                err => observer.error(err),
                () => observer.complete()
            );
            return () => subscription.unsubscribe();
        });
    }

    public getResource(name: string, resource: models.ResourceNode): Promise<models.State> {
        return requests
            .get(`/applications/${name}/resource`)
            .query({
                name: resource.name,
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group
            })
            .then(res => res.body as {manifest: string})
            .then(res => JSON.parse(res.manifest) as models.State);
    }

    public getResourceActions(name: string, resource: models.ResourceNode): Promise<models.ResourceAction[]> {
        return requests
            .get(`/applications/${name}/resource/actions`)
            .query({
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group
            })
            .then(res => (res.body.actions as models.ResourceAction[]) || []);
    }

    public runResourceAction(name: string, resource: models.ResourceNode, action: string): Promise<models.ResourceAction[]> {
        return requests
            .post(`/applications/${name}/resource/actions`)
            .query({
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group
            })
            .send(JSON.stringify(action))
            .then(res => (res.body.actions as models.ResourceAction[]) || []);
    }

    public patchResource(name: string, resource: models.ResourceNode, patch: string, patchType: string): Promise<models.State> {
        return requests
            .post(`/applications/${name}/resource`)
            .query({
                name: resource.name,
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group,
                patchType
            })
            .send(JSON.stringify(patch))
            .then(res => res.body as {manifest: string})
            .then(res => JSON.parse(res.manifest) as models.State);
    }

    public deleteResource(applicationName: string, resource: models.ResourceNode, force: boolean): Promise<any> {
        return requests
            .delete(`/applications/${applicationName}/resource`)
            .query({
                name: resource.name,
                namespace: resource.namespace,
                resourceName: resource.name,
                version: resource.version,
                kind: resource.kind,
                group: resource.group,
                force
            })
            .send()
            .then(() => true);
    }

    public events(applicationName: string): Promise<models.Event[]> {
        return requests
            .get(`/applications/${applicationName}/events`)
            .send()
            .then(res => (res.body as models.EventList).items || []);
    }

    public resourceEvents(
        applicationName: string,
        resource: {
            namespace: string;
            name: string;
            uid: string;
        }
    ): Promise<models.Event[]> {
        return requests
            .get(`/applications/${applicationName}/events`)
            .query({
                resourceUID: resource.uid,
                resourceNamespace: resource.namespace,
                resourceName: resource.name
            })
            .send()
            .then(res => (res.body as models.EventList).items || []);
    }

    public terminateOperation(applicationName: string): Promise<boolean> {
        return requests
            .delete(`/applications/${applicationName}/operation`)
            .send()
            .then(() => true);
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
}
