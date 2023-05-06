import * as deepMerge from 'deepmerge';
import {Observable} from 'rxjs';
import {map, repeat, retry} from 'rxjs/operators';

import * as models from '../models';
import {isValidURL} from '../utils';
import requests from './requests';

interface QueryOptions {
    fields: string[];
    exclude?: boolean;
    selector?: string;
    appSetNamespace?: string;
}

function optionsToSearch(options?: QueryOptions) {
    if (options) {
        return {fields: (options.exclude ? '-' : '') + options.fields.join(','), selector: options.selector || '', appNamespace: options.appSetNamespace || ''};
    }
    return {};
}

export class ApplicationSetsService {
    public list(options?: QueryOptions): Promise<models.ApplicationSetList> {
        return requests
            .get('/applicationsets')
            .query({...optionsToSearch(options)})
            .then(res => res.body as models.ApplicationSetList)
            .then(list => {
                list.items = (list.items || []).map(app => this.parseAppSetFields(app));
                return list;
            });
    }

    public get(name: string, appNamespace: string, refresh?: 'normal' | 'hard'): Promise<models.ApplicationSet> {
        const query: {[key: string]: string} = {};
        if (refresh) {
            query.refresh = refresh;
        }
        if (appNamespace) {
            query.appNamespace = appNamespace;
        }
        return requests
            .get(`/applicationsets/${name}`)
            .query(query)
            .then(res => this.parseAppSetFields(res.body));
    }


    public resourceTree(name: string, appNamespace: string): Promise<models.ApplicationTree> {
        return requests
            .get(`/applicationsets/${name}/resource-tree`)
            .query({appNamespace})
            .then(res => res.body as models.ApplicationTree);
    }

    public watchResourceTree(name: string, appNamespace: string): Observable<models.ApplicationTree> {
        return requests
            .loadEventSource(`/stream/applicationsets/${name}/resource-tree?appNamespace=${appNamespace}`)
            .pipe(map(data => JSON.parse(data).result as models.ApplicationTree));
    }

    public managedResources(name: string, appNamespace: string, options: {id?: models.ResourceID; fields?: string[]} = {}): Promise<models.ResourceDiff[]> {
        return requests
            .get(`/applicationsets/${name}/managed-resources`)
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
            .get(`/applicationsets/${name}/manifests`)
            .query({name, revision})
            .then(res => res.body as models.ManifestResponse);
    }

    public updateSpec(appName: string, appNamespace: string, spec: models.ApplicationSpec): Promise<models.ApplicationSpec> {
        return requests
            .put(`/applicationsets/${appName}/spec`)
            .send(spec)
            .then(res => res.body as models.ApplicationSpec);
    }

    public update(app: models.Application, query: {validate?: boolean} = {}): Promise<models.ApplicationSet> {
        return requests
            .put(`/applicationsets/${app.metadata.name}`)
            .query(query)
            .send(app)
            .then(res => this.parseAppSetFields(res.body));
    }

    public create(app: models.Application): Promise<models.ApplicationSet> {
        // Namespace may be specified in the app name. We need to parse and
        // handle it accordingly.
        if (app.metadata.name.includes('/')) {
            const nns = app.metadata.name.split('/', 2);
            app.metadata.name = nns[1];
            app.metadata.namespace = nns[0];
        }
        return requests
            .post(`/applicationsets`)
            .send(app)
            .then(res => this.parseAppSetFields(res.body));
    }

    public delete(name: string, appNamespace: string, propagationPolicy: string): Promise<boolean> {
        let cascade = true;
        if (propagationPolicy === 'non-cascading') {
            propagationPolicy = '';
            cascade = false;
        }
        return requests
            .delete(`/applicationsets/${name}`)
            .query({
                cascade,
                propagationPolicy,
                appNamespace
            })
            .send({})
            .then(() => true);
    }

    public watch(query?: {name?: string; resourceVersion?: string; projects?: string[]; appNamespace?: string}, options?: QueryOptions): Observable<models.ApplicationSetWatchEvent> {
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
            search.set('fields', searchOptions.fields);
            search.set('selector', searchOptions.selector);
            search.set('appNamespace', searchOptions.appNamespace);
            query?.projects?.forEach(project => search.append('projects', project));
        }
        const searchStr = search.toString();
        const url = `/stream/applicationsets${(searchStr && '?' + searchStr) || ''}`;
        return requests
            .loadEventSource(url)
            .pipe(repeat())
            .pipe(retry())
            .pipe(map(data => JSON.parse(data).result as models.ApplicationSetWatchEvent))
            .pipe(
                map(watchEvent => {
                    watchEvent.applicationSet = this.parseAppSetFields(watchEvent.applicationSet);
                    return watchEvent;
                })
            );
    }

    public getResource(name: string, appNamespace: string, resource: models.ResourceNode): Promise<models.State> {
        return requests
            .get(`/applicationsets/${name}/resource`)
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

    
    private parseAppSetFields(data: any): models.ApplicationSet {
        data = deepMerge(
            {
                apiVersion: 'argoproj.io/v1alpha1',
                kind: 'ApplicationSet',
            },
            data
        );

        return data as models.ApplicationSet;
    }
}
