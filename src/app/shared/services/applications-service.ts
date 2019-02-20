import { Observable } from 'rxjs';

import * as models from '../models';
import requests from './requests';

interface QueryOptions {
    fields: string[];
    exclude?: boolean;
}

function optionsToSearch(options?: QueryOptions) {
    if (options) {
        return { fields: (options.exclude ? '-' : '') + options.fields.join(',') };
    }
    return {};
}

export class ApplicationsService {
    public list(projects: string[], options?: QueryOptions): Promise<models.Application[]> {
        return requests.get('/applications').query({ project: projects, ...optionsToSearch(options) }).then((res) => res.body as models.ApplicationList).then((list) => {
            return (list.items || []).map((app) => this.parseAppFields(app));
        });
    }

    public get(name: string, refresh?: 'normal' | 'hard'): Promise<models.Application> {
        const query: {[key: string]: string} = {};
        if (refresh) {
            query.refresh = refresh;
        }
        return requests.get(`/applications/${name}`).query(query).then((res) => this.parseAppFields(res.body));
    }

    public resourceTree(name: string): Promise<models.ResourceNode[]> {
        return requests.get(`/applications/${name}/resource-tree`).then((res) => res.body.items as models.ResourceNode[] || []);
    }

    public managedResources(name: string): Promise<models.ResourceDiff[]> {
        return requests.get(`/applications/${name}/managed-resources`).then((res) => res.body.items as any[] || []).then((items) => {
            items.forEach((item) => {
                item.liveState = JSON.parse(item.liveState);
                item.targetState = JSON.parse(item.targetState);
            });
            return items as models.ResourceDiff[];
        });
    }

    public getManifest(name: string, revision: string): Promise<models.ManifestResponse> {
        return requests.get(`/applications/${name}/manifests`).query({name, revision}).then((res) => res.body as models.ManifestResponse);
    }

    public updateSpec(appName: string, spec: models.ApplicationSpec): Promise<models.ApplicationSpec> {
        return requests.put(`/applications/${appName}/spec`).send(spec).then((res) => res.body as models.ApplicationSpec);
    }

    public create(name: string, project: string, source: models.ApplicationSource, destination?: models.ApplicationDestination): Promise<models.Application> {
        return requests.post(`/applications`).send({
            metadata: { name },
            spec: { source, destination, project },
        }).then((res) => this.parseAppFields(res.body));
    }

    public delete(name: string, cascade: boolean): Promise<boolean> {
        return requests.delete(`/applications/${name}`).query({cascade}).send({}).then(() => true);
    }

    public watch(query?: {name: string}, options?: QueryOptions): Observable<models.ApplicationWatchEvent> {
        const search = new URLSearchParams();
        if (query) {
            search.set('name', query.name);
        }
        if (options) {
            const searchOptions = optionsToSearch(options);
            search.set('fields', searchOptions.fields);
        }
        const searchStr = search.toString();
        const url = `/stream/applications${searchStr && '?' + searchStr || ''}` ;
        return requests.loadEventSource(url).repeat().retry().map((data) => JSON.parse(data).result as models.ApplicationWatchEvent).map((watchEvent) => {
            watchEvent.application = this.parseAppFields(watchEvent.application);
            return watchEvent;
        });
    }

    public sync(name: string, revision: string, prune: boolean, dryRun: boolean, resources: models.SyncOperationResource[]): Promise<boolean> {
        return requests.post(`/applications/${name}/sync`).send({revision, prune: !!prune, dryRun: !!dryRun, resources}).then(() => true);
    }

    public rollback(name: string, id: number): Promise<boolean> {
        return requests.post(`/applications/${name}/rollback`).send({id}).then(() => true);
    }

    public getContainerLogs(applicationName: string, namespace: string, podName: string, containerName: string): Observable<models.LogEntry> {
        return requests.loadEventSource(`/applications/${applicationName}/pods/${podName}/logs?container=${containerName}&follow=true&namespace=${namespace}`).repeat().retry().map(
            (data) => JSON.parse(data).result as models.LogEntry);
    }

    public getResource(name: string, resource: models.ResourceNode): Promise<models.State> {
        return requests.get(`/applications/${name}/resource`).query({
            name: resource.name,
            namespace: resource.namespace,
            resourceName: resource.name,
            version: resource.version,
            kind: resource.kind,
            group: resource.group,
        }).then((res) => res.body as { manifest: string }).then((res) => JSON.parse(res.manifest) as models.State);
    }

    public deleteResource(applicationName: string, resource: models.ResourceNode, force: boolean): Promise<any> {
        return requests.delete(`/applications/${applicationName}/resource`).query({
            name: resource.name,
            namespace: resource.namespace,
            resourceName: resource.name,
            version: resource.version,
            kind: resource.kind,
            group: resource.group,
            force,
        }).send().then(() => true);
    }

    public events(applicationName: string): Promise<models.Event[]> {
        return requests.get(`/applications/${applicationName}/events`).send().then((res) => (res.body as models.EventList).items || []);
    }

    public resourceEvents(applicationName: string, resource: {
        namespace: string,
        name: string,
        uid: string,
    }): Promise<models.Event[]> {
        return requests.get(`/applications/${applicationName}/events`).query({
            resourceUID: resource.uid,
            resourceNamespace: resource.namespace,
            resourceName: resource.name,
        }).send().then((res) => (res.body as models.EventList).items || []);
    }

    public terminateOperation(applicationName: string): Promise<boolean> {
        return requests.delete(`/applications/${applicationName}/operation`).send().then(() => true);
    }

    private parseAppFields(data: any): models.Application {
        const app = data as models.Application;
        app.spec.project = app.spec.project || 'default';
        app.kind = app.kind || 'Application';
        app.status.resources = app.status.resources || [];
        return app;
    }
}
