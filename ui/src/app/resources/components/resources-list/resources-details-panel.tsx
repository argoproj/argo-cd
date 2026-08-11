import {SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {combineLatest, from, merge} from 'rxjs';
import {delay, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';
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

function buildFallbackTree(application: appModels.Application): appModels.ApplicationTree {
    return {
        nodes:
            application.status?.resources?.map((res: appModels.ResourceStatus) => ({
                ...res,
                parentRefs: [] as appModels.ResourceRef[],
                info: [] as appModels.InfoItem[],
                resourceVersion: '',
                uid: ''
            })) || [],
        orphanedNodes: [],
        hosts: []
    };
}

/** Sliding resource details panel for the global Resources page (read-only app context). */
export const ResourcesDetailsPanel = (props: {node: string | null; detailsApp: string | null}) => {
    const appContext = React.useContext(Context);
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
        return from(services.applications.get(appName, appNamespace, 'application')).pipe(
            mergeMap(application => {
                const fallbackTree = buildFallbackTree(application as appModels.Application);
                const emptyTree: appModels.ApplicationTree = {nodes: [], orphanedNodes: [], hosts: []};
                return combineLatest([
                    from([application]),
                    merge(
                        from([fallbackTree]),
                        from(services.applications.resourceTree(appName, appNamespace, 'application')).pipe(map(tree => tree || emptyTree)),
                        AppUtils.handlePageVisibility(() =>
                            services.applications
                                .watchResourceTree(appName, appNamespace, 'application')
                                .pipe(repeat())
                                .pipe(retryWhen(errors => errors.pipe(delay(500))))
                        )
                    )
                ]).pipe(map(([app, tree]) => ({application: app, tree: tree || fallbackTree})));
            })
        );
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
                                updateApp={async () => {
                                    throw new Error('Application updates are not available from the Resources page');
                                }}
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
