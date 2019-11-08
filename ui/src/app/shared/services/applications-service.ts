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
    public list(projects: string[], options?: QueryOptions): Promise<models.Application[]> {
        return requests
            .get('/applications')
            .query({project: projects, ...optionsToSearch(options)})
            .then(res => res.body as models.ApplicationList)
            .then(list => {
                return (list.items || []).map(app => this.parseAppFields(app));
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

    public managedResources(name: string): Promise<models.ResourceDiff[]> {
        return requests
            .get(`/applications/${name}/managed-resources`)
            .then(res => (res.body.items as any[]) || [])
            .then(items => {
                items.forEach(item => {
                    item.liveState = JSON.parse(item.liveState);
                    item.targetState = JSON.parse(item.targetState);
                });
                return items as models.ResourceDiff[];
            });
    }

    public getManifest(name: string, revision: string): Promise<models.ManifestResponse> {
        return requests
            .get(`/applications/${name}/manifests`)
            .query({name, revision})
            .then(res => res.body as models.ManifestResponse);
    }

    public async updateSpec(appName: string, spec: models.ApplicationSpec): Promise<models.ApplicationSpec> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .put(`/applications/${appName}/spec`)
            .set(requests.csrfHeaderName, csrfToken)
            .send(spec)
            .then(res => res.body as models.ApplicationSpec);
    }

    public async update(app: models.Application): Promise<models.Application> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .put(`/applications/${app.metadata.name}`)
            .set(requests.csrfHeaderName, csrfToken)
            .send(app)
            .then(res => res.body as models.Application);
    }

    public async create(app: models.Application): Promise<models.Application> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post(`/applications`)
            .set(requests.csrfHeaderName, csrfToken)
            .send(app)
            .then(res => this.parseAppFields(res.body));
    }

    public async delete(name: string, cascade: boolean): Promise<boolean> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .delete(`/applications/${name}`)
            .set(requests.csrfHeaderName, csrfToken)
            .query({cascade})
            .send({})
            .then(() => true);
    }

    public watch(query?: {name: string}, options?: QueryOptions): Observable<models.ApplicationWatchEvent> {
        const search = new URLSearchParams();
        if (query) {
            search.set('name', query.name);
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

    public async sync(name: string, revision: string, prune: boolean, dryRun: boolean, strategy: models.SyncStrategy, resources: models.SyncOperationResource[]): Promise<boolean> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post(`/applications/${name}/sync`)
            .set(requests.csrfHeaderName, csrfToken)
            .send({revision, prune: !!prune, dryRun: !!dryRun, strategy, resources})
            .then(() => true);
    }

    public async rollback(name: string, id: number): Promise<boolean> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post(`/applications/${name}/rollback`)
            .set(requests.csrfHeaderName, csrfToken)
            .send({id})
            .then(() => true);
    }

    public getContainerLogs(applicationName: string, namespace: string, podName: string, containerName: string): Observable<models.LogEntry> {
        return requests
            .loadEventSource(`/applications/${applicationName}/pods/${podName}/logs?container=${containerName}&follow=true&namespace=${namespace}`)
            .map(data => JSON.parse(data).result as models.LogEntry);
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

    public async runResourceAction(name: string, resource: models.ResourceNode, action: string): Promise<models.ResourceAction[]> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post(`/applications/${name}/resource/actions`)
            .set(requests.csrfHeaderName, csrfToken)
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

    public async patchResource(name: string, resource: models.ResourceNode, patch: string, patchType: string): Promise<models.State> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post(`/applications/${name}/resource`)
            .set(requests.csrfHeaderName, csrfToken)
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

    public async deleteResource(applicationName: string, resource: models.ResourceNode, force: boolean): Promise<any> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .delete(`/applications/${applicationName}/resource`)
            .set(requests.csrfHeaderName, csrfToken)
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

    public async terminateOperation(applicationName: string): Promise<boolean> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .delete(`/applications/${applicationName}/operation`)
            .set(requests.csrfHeaderName, csrfToken)
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
