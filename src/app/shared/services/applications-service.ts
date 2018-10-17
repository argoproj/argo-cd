import { Observable } from 'rxjs';

import * as models from '../models';
import requests from './requests';

export interface ManifestQuery {
    name: string;
    revision?: string;
}
export interface ManifestResponse {
    manifests: string[];
    namespace: string;
    server: string;
    revision: string;
    params: models.ComponentParameter[];
}

export class ApplicationsService {
    public list(projects: string[]): Promise<models.Application[]> {
        return requests.get('/applications').query({ project: projects }).then((res) => res.body as models.ApplicationList).then((list) => {
            return (list.items || []).map((app) => this.parseAppFields(app));
        });
    }

    public get(name: string, refresh = false): Promise<models.Application> {
        return requests.get(`/applications/${name}`).query({refresh}).then((res) => this.parseAppFields(res.body));
    }

    public getManifest(name: string, revision: string): Promise<ManifestResponse> {
        return requests.get(`/applications/${name}/manifests`).query({name, revision} as ManifestQuery).then((res) => res.body as ManifestResponse);
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

    public watch(query?: {name: string}): Observable<models.ApplicationWatchEvent> {
        let url = '/stream/applications';
        if (query) {
            url = `${url}?name=${query.name}`;
        }
        return requests.loadEventSource(url).repeat().retry().map((data) => JSON.parse(data).result as models.ApplicationWatchEvent).map((watchEvent) => {
            watchEvent.application = this.parseAppFields(watchEvent.application);
            return watchEvent;
        });
    }

    public sync(name: string, revision: string, prune: boolean): Promise<boolean> {
        return requests.post(`/applications/${name}/sync`).send({revision, prune: !!prune}).then(() => true);
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
