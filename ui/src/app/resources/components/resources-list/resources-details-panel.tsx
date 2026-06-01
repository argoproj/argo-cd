import {SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {BehaviorSubject, combineLatest, from, merge} from 'rxjs';
import {delay, filter, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';
import {DataLoader} from '../../../shared/components';
import {AppContext, Context} from '../../../shared/context';
import * as appModels from '../../../shared/models';
import {services} from '../../../shared/services';
import {NodeInfo} from '../../../applications/components/application-details/application-details';
import {ResourceDetails} from '../../../applications/components/resource-details/resource-details';
import * as AppUtils from '../../../applications/components/utils';

function parseDetailsApp(detailsApp: string): {appNamespace: string; appName: string} | null {
    const slash = detailsApp.indexOf('/');
    if (slash <= 0) {
        return null;
    }
    return {appNamespace: detailsApp.slice(0, slash), appName: detailsApp.slice(slash + 1)};
}

function findResourceNode(application: appModels.Application, tree: appModels.ApplicationTree, selectedNodeKey: string): appModels.ResourceNode | null {
    const nodeByKey = new Map<string, appModels.ResourceNode>();
    (tree.nodes || []).concat(tree.orphanedNodes || []).forEach(node => nodeByKey.set(AppUtils.nodeKey(node), node));
    application.status?.resources?.forEach(res => {
        const key = AppUtils.nodeKey(res);
        if (!nodeByKey.has(key)) {
            nodeByKey.set(key, AppUtils.resourceStatusToResourceNode(res));
        }
    });
    return nodeByKey.get(selectedNodeKey) || null;
}

export const ResourcesDetailsPanel = (props: {node: string | null; detailsApp: string | null}) => {
    const appContext = React.useContext(Context);
    const appChanged = React.useRef(new BehaviorSubject<appModels.Application>(null));
    const parsed = props.detailsApp ? parseDetailsApp(props.detailsApp) : null;
    const isShown = !!props.node && !!parsed;

    const closePanel = () => {
        appContext.navigation.goto('.', {node: null, tab: null, detailsApp: null}, {replace: true});
    };

    const load = () => {
        if (!parsed) {
            return from([]);
        }
        const {appName, appNamespace} = parsed;
        const fallbackTree: appModels.ApplicationTree = {
            nodes: [],
            orphanedNodes: [],
            hosts: []
        };
        return from(services.applications.get(appName, appNamespace, 'application')).pipe(
            mergeMap(application => {
                const fallback: appModels.ApplicationTree = {
                    nodes:
                        application.status?.resources?.map((res: appModels.ResourceStatus) => ({
                            ...res,
                            parentRefs: [],
                            info: [],
                            resourceVersion: '',
                            uid: ''
                        })) || [],
                    orphanedNodes: [],
                    hosts: []
                };
                return combineLatest([
                    merge(from([application]), appChanged.current.pipe(filter(item => !!item))),
                    merge(
                        from([fallback]),
                        from(services.applications.resourceTree(appName, appNamespace, 'application')).pipe(map(tree => tree || fallbackTree)),
                        AppUtils.handlePageVisibility(() =>
                            services.applications
                                .watchResourceTree(appName, appNamespace, 'application')
                                .pipe(repeat())
                                .pipe(retryWhen(errors => errors.pipe(delay(500))))
                        )
                    )
                ]).pipe(map(([app, tree]) => ({application: app, tree: tree || fallback})));
            })
        );
    };

    const updateApp = async (app: appModels.Application, query: {validate?: boolean}) => {
        const latestApp = await services.applications.get(app.metadata.name, app.metadata.namespace, 'application');
        latestApp.metadata.labels = app.metadata.labels;
        latestApp.metadata.annotations = app.metadata.annotations;
        latestApp.spec = app.spec;
        const updatedApp = await services.applications.update(latestApp, query);
        appChanged.current.next(updatedApp);
    };

    return (
        <SlidingPanel isShown={isShown} onClose={closePanel}>
            {isShown && parsed && (
                <DataLoader load={load} input={`${parsed.appNamespace}/${parsed.appName}/${props.node}`}>
                    {({application, tree}: {application: appModels.Application; tree: appModels.ApplicationTree}) => {
                        const selectedNodeKey = NodeInfo(props.node).key;
                        const selectedNode = findResourceNode(application, tree, selectedNodeKey);
                        if (!selectedNode) {
                            return <div className='p-3'>Resource not found in application tree.</div>;
                        }
                        return (
                            <ResourceDetails
                                tree={tree}
                                application={application}
                                isAppSelected={false}
                                updateApp={updateApp}
                                selectedNode={selectedNode}
                                appCxt={{...appContext, apis: appContext} as unknown as AppContext}
                            />
                        );
                    }}
                </DataLoader>
            )}
        </SlidingPanel>
    );
};
