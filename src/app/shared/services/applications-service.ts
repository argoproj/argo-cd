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

    public get(name: string, refresh = false): Promise<models.Application> {
        return requests.get(`/applications/${name}`).query({refresh}).then((res) => this.parseAppFields(res.body));
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

    public sync(name: string, revision: string, prune: boolean, resources: models.SyncOperationResource[]): Promise<boolean> {
        return requests.post(`/applications/${name}/sync`).send({revision, prune: !!prune, resources}).then(() => true);
    }

    public rollback(name: string, id: number): Promise<boolean> {
        return requests.post(`/applications/${name}/rollback`).send({id}).then(() => true);
    }

    public getContainerLogs(applicationName: string, podName: string, containerName: string): Observable<models.LogEntry> {
        return requests.loadEventSource(`/applications/${applicationName}/pods/${podName}/logs?container=${containerName}&follow=true`).repeat().retry().map(
            (data) => JSON.parse(data).result as models.LogEntry);
    }

    public deleteResource(applicationName: string, resourceName: string, apiVersion: string, kind: string): Promise<any> {
        return requests.delete(`/applications/${applicationName}/resource`).query({resourceName, apiVersion, kind}).send().then(() => true);
    }

    public events(applicationName: string): Promise<models.Event[]> {
        return this.resourceEvents(applicationName, null, null);
    }

    public resourceEvents(applicationName: string, resourceUID: string, resourceName: string): Promise<models.Event[]> {
        return requests.get(`/applications/${applicationName}/events`).query({resourceName, resourceUID}).send().then((res) => (res.body as models.EventList).items || []);
    }

    public terminateOperation(applicationName: string): Promise<boolean> {
        return requests.delete(`/applications/${applicationName}/operation`).send().then(() => true);
    }

    private parseAppFields(data: any): models.Application {
        const app = data as models.Application;
        app.spec.project = app.spec.project || 'default';
        (app.status.comparisonResult.resources || []).forEach((resource) => {
            resource.liveState = JSON.parse(resource.liveState as any);
            resource.targetState = JSON.parse(resource.targetState as any);
            function parseResourceNodes(node: models.ResourceNode) {
                node.state = JSON.parse(node.state as any);
                (node.children || []).forEach(parseResourceNodes);
            }
            (resource.childLiveResources || []).forEach((node) => {
                parseResourceNodes(node);
            });
        });
        app.kind = app.kind || 'Application';
        return app;
    }
}
