import {DataLoader, DropDown, Tab, Tabs} from 'argo-ui';
import * as React from 'react';
import {useState} from 'react';
import {EventsList, YamlEditor} from '../../../shared/components';
import * as models from '../../../shared/models';
import {ErrorBoundary} from '../../../shared/components/error-boundary/error-boundary';
import {AppContext, Context} from '../../../shared/context';
import {Application, ApplicationTree, Event, ResourceNode, State, SyncStatuses} from '../../../shared/models';
import {services} from '../../../shared/services';
import {ResourceTabExtension} from '../../../shared/services/extensions-service';
import {NodeInfo, SelectNode} from '../application-details/application-details';
import {ApplicationNodeInfo} from '../application-node-info/application-node-info';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {ApplicationResourceEvents} from '../application-resource-events/application-resource-events';
import {ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';
import {ApplicationSummary} from '../application-summary/application-summary';
import {PodsLogsViewer} from '../pod-logs-viewer/pod-logs-viewer';
import {PodTerminalViewer} from '../pod-terminal-viewer/pod-terminal-viewer';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import * as AppUtils from '../utils';
import './resource-details.scss';

const jsonMergePatch = require('json-merge-patch');

interface ResourceDetailsProps {
    selectedNode: ResourceNode;
    updateApp: (app: Application, query: {validate?: boolean}) => Promise<any>;
    application: Application;
    isAppSelected: boolean;
    tree: ApplicationTree;
    tab?: string;
    appCxt: AppContext;
}

export const ResourceDetails = (props: ResourceDetailsProps) => {
    const {selectedNode, updateApp, application, isAppSelected, tree} = {...props};
    const [activeContainer, setActiveContainer] = useState();
    const appContext = React.useContext(Context);
    const tab = new URLSearchParams(appContext.history.location.search).get('tab');
    const selectedNodeInfo = NodeInfo(new URLSearchParams(appContext.history.location.search).get('node'));
    const selectedNodeKey = selectedNodeInfo.key;
    const [pageNumber, setPageNumber] = React.useState(0);
    const [collapsedSources, setCollapsedSources] = React.useState(new Array<boolean>()); // For Sources tab to save collapse states
    const handleCollapse = (i: number, isCollapsed: boolean) => {
        const v = collapsedSources.slice();
        v[i] = isCollapsed;
        setCollapsedSources(v);
    };

    const getResourceTabs = (
        node: ResourceNode,
        state: State,
        podState: State,
        events: Event[],
        extensionTabs: ResourceTabExtension[],
        tabs: Tab[],
        execEnabled: boolean,
        execAllowed: boolean,
        logsAllowed: boolean
    ) => {
        if (!node || node === undefined) {
            return [];
        }
        if (state) {
            const numErrors = events.filter(event => event.type !== 'Normal').reduce((total, event) => total + event.count, 0);
            tabs.push({
                title: 'EVENTS',
                icon: 'fa fa-calendar-alt',
                badge: (numErrors > 0 && numErrors) || null,
                key: 'events',
                content: (
                    <div className='application-resource-events'>
                        <EventsList events={events} />
                    </div>
                )
            });
        }
        if (podState && podState.metadata && podState.spec) {
            const containerGroups = [
                {
                    offset: 0,
                    title: 'CONTAINERS',
                    containers: podState.spec.containers || []
                }
            ];
            if (podState.spec.initContainers?.length > 0) {
                containerGroups.push({
                    offset: (podState.spec.containers || []).length,
                    title: 'INIT CONTAINERS',
                    containers: podState.spec.initContainers || []
                });
            }

            const onClickContainer = (group: any, i: number, activeTab: string) => {
                setActiveContainer(group.offset + i);
                SelectNode(selectedNodeKey, activeContainer, activeTab, appContext);
            };

            if (logsAllowed) {
                tabs = tabs.concat([
                    {
                        key: 'logs',
                        icon: 'fa fa-align-left',
                        title: 'LOGS',
                        content: (
                            <div className='application-details__tab-content-full-height'>
                                <PodsLogsViewer
                                    podName={(state.kind === 'Pod' && state.metadata.name) || ''}
                                    group={node.group}
                                    kind={node.kind}
                                    name={node.name}
                                    namespace={podState.metadata.namespace}
                                    applicationName={application.metadata.name}
                                    applicationNamespace={application.metadata.namespace}
                                    containerName={AppUtils.getContainerName(podState, activeContainer)}
                                    containerGroups={containerGroups}
                                    onClickContainer={onClickContainer}
                                />
                            </div>
                        )
                    }
                ]);
            }
            if (selectedNode?.kind === 'Pod' && execEnabled && execAllowed) {
                tabs = tabs.concat([
                    {
                        key: 'exec',
                        icon: 'fa fa-terminal',
                        title: 'Terminal',
                        content: (
                            <PodTerminalViewer
                                applicationName={application.metadata.name}
                                applicationNamespace={application.metadata.namespace}
                                projectName={application.spec.project}
                                podState={podState}
                                selectedNode={selectedNode}
                                containerName={AppUtils.getContainerName(podState, activeContainer)}
                                onClickContainer={onClickContainer}
                            />
                        )
                    }
                ]);
            }
        }
        if (state) {
            extensionTabs.forEach((tabExtensions, i) => {
                tabs.push({
                    title: tabExtensions.title,
                    key: `extension-${i}`,
                    content: (
                        <ErrorBoundary message={`Something went wrong with Extension for ${state?.kind || 'resource of unknown kind'}`}>
                            <tabExtensions.component tree={tree} resource={state} application={application} />
                        </ErrorBoundary>
                    ),
                    icon: tabExtensions.icon
                });
            });
        }
        return tabs;
    };

    const getApplicationTabs = () => {
        const tabs: Tab[] = [
            {
                title: 'SUMMARY',
                key: 'summary',
                content: <ApplicationSummary app={application} updateApp={(app, query: {validate?: boolean}) => updateApp(app, query)} />
            },
            {
                title: application.spec.sources === undefined ? 'PARAMETERS' : 'SOURCES',
                key: 'parameters',
                content: (
                    <ApplicationParameters
                        save={(app: models.Application, query: {validate?: boolean}) => updateApp(app, query)}
                        application={application}
                        pageNumber={pageNumber}
                        setPageNumber={setPageNumber}
                        collapsedSources={collapsedSources}
                        handleCollapse={handleCollapse}
                        appContext={props.appCxt}
                    />
                )
            },
            {
                title: 'MANIFEST',
                key: 'manifest',
                content: (
                    <YamlEditor
                        minHeight={800}
                        input={application.spec}
                        onSave={async patch => {
                            const spec = JSON.parse(JSON.stringify(application.spec));
                            return services.applications.updateSpec(application.metadata.name, application.metadata.namespace, jsonMergePatch.apply(spec, JSON.parse(patch)));
                        }}
                    />
                )
            }
        ];

        if (application.status.sync.status !== SyncStatuses.Synced) {
            tabs.push({
                icon: 'fa fa-file-medical',
                title: 'DIFF',
                key: 'diff',
                content: (
                    <DataLoader
                        key='diff'
                        load={async () =>
                            await services.applications.managedResources(application.metadata.name, application.metadata.namespace, {
                                fields: ['items.normalizedLiveState', 'items.predictedLiveState', 'items.group', 'items.kind', 'items.namespace', 'items.name']
                            })
                        }>
                        {managedResources => <ApplicationResourcesDiff states={managedResources} />}
                    </DataLoader>
                )
            });
        }

        tabs.push({
            title: 'EVENTS',
            key: 'event',
            content: <ApplicationResourceEvents applicationName={application.metadata.name} applicationNamespace={application.metadata.namespace} />
        });

        const extensionTabs = services.extensions.getResourceTabs('argoproj.io', 'Application').map((ext, i) => ({
            title: ext.title,
            key: `extension-${i}`,
            content: <ext.component resource={application} tree={tree} application={application} />,
            icon: ext.icon
        }));

        return tabs.concat(extensionTabs);
    };

    const extensions = selectedNode?.kind ? services.extensions.getResourceTabs(selectedNode?.group || '', selectedNode?.kind) : [];

    return (
        <div style={{width: '100%', height: '100%'}}>
            {selectedNode && (
                <DataLoader
                    noLoaderOnInputChange={true}
                    input={selectedNode.resourceVersion}
                    load={async () => {
                        const managedResources = await services.applications.managedResources(application.metadata.name, application.metadata.namespace, {
                            id: {
                                name: selectedNode.name,
                                namespace: selectedNode.namespace,
                                kind: selectedNode.kind,
                                group: selectedNode.group
                            }
                        });
                        const controlled = managedResources.find(item => AppUtils.isSameNode(selectedNode, item));
                        const summary = application.status.resources.find(item => AppUtils.isSameNode(selectedNode, item));
                        const controlledState = (controlled && summary && {summary, state: controlled}) || null;
                        const resQuery = {...selectedNode};
                        if (controlled && controlled.targetState) {
                            resQuery.version = AppUtils.parseApiVersion(controlled.targetState.apiVersion).version;
                        }
                        const liveState = await services.applications.getResource(application.metadata.name, application.metadata.namespace, resQuery).catch(() => null);
                        const events =
                            (liveState &&
                                (await services.applications.resourceEvents(application.metadata.name, application.metadata.namespace, {
                                    name: liveState.metadata.name,
                                    namespace: liveState.metadata.namespace,
                                    uid: liveState.metadata.uid
                                }))) ||
                            [];
                        let podState: State;
                        let childResources: models.ResourceNode[] = [];
                        if (selectedNode.kind === 'Pod') {
                            podState = liveState;
                        } else {
                            const childPod = AppUtils.findChildPod(selectedNode, tree);
                            if (childPod) {
                                podState = await services.applications.getResource(application.metadata.name, application.metadata.namespace, childPod).catch(() => null);
                            }
                            childResources = AppUtils.findChildResources(selectedNode, tree);
                        }

                        const settings = await services.authService.settings();
                        const execEnabled = settings.execEnabled;
                        const logsAllowed = await services.accounts.canI('logs', 'get', application.spec.project + '/' + application.metadata.name);
                        const execAllowed = execEnabled && (await services.accounts.canI('exec', 'create', application.spec.project + '/' + application.metadata.name));
                        const links = await services.applications.getResourceLinks(application.metadata.name, application.metadata.namespace, selectedNode).catch(() => null);
                        return {controlledState, liveState, events, podState, execEnabled, execAllowed, logsAllowed, links, childResources};
                    }}>
                    {data => (
                        <React.Fragment>
                            <div className='resource-details__header'>
                                <div style={{display: 'flex', flexDirection: 'column', marginRight: '15px', alignItems: 'center', fontSize: '12px'}}>
                                    <ResourceIcon kind={selectedNode.kind} />
                                    {ResourceLabel({kind: selectedNode.kind})}
                                </div>
                                <h1>{selectedNode.name}</h1>
                                {data.controlledState && (
                                    <React.Fragment>
                                        <span style={{marginRight: '5px'}}>
                                            <AppUtils.ComparisonStatusIcon status={data.controlledState.summary.status} resource={data.controlledState.summary} />
                                        </span>
                                    </React.Fragment>
                                )}
                                {(selectedNode as ResourceTreeNode).health && <AppUtils.HealthStatusIcon state={(selectedNode as ResourceTreeNode).health} />}
                                <button
                                    onClick={() => appContext.navigation.goto('.', {deploy: AppUtils.nodeKey(selectedNode)}, {replace: true})}
                                    style={{marginLeft: 'auto', marginRight: '5px'}}
                                    className='argo-button argo-button--base'>
                                    <i className='fa fa-sync-alt' /> <span className='show-for-large'>SYNC</span>
                                </button>
                                <button
                                    onClick={() => AppUtils.deletePopup(appContext, selectedNode, application, !!data.controlledState, data.childResources)}
                                    style={{marginRight: '5px'}}
                                    className='argo-button argo-button--base'>
                                    <i className='fa fa-trash' /> <span className='show-for-large'>DELETE</span>
                                </button>
                                <DropDown
                                    isMenu={true}
                                    anchor={() => (
                                        <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                            <i className='fa fa-ellipsis-v' />
                                        </button>
                                    )}>
                                    {() => AppUtils.renderResourceActionMenu(selectedNode, application, appContext)}
                                </DropDown>
                            </div>
                            <Tabs
                                navTransparent={true}
                                tabs={getResourceTabs(
                                    selectedNode,
                                    data.liveState,
                                    data.podState,
                                    data.events,
                                    extensions,
                                    [
                                        {
                                            title: 'SUMMARY',
                                            icon: 'fa fa-file-alt',
                                            key: 'summary',
                                            content: (
                                                <ApplicationNodeInfo
                                                    application={application}
                                                    live={data.liveState}
                                                    controlled={data.controlledState}
                                                    node={selectedNode}
                                                    links={data.links}
                                                />
                                            )
                                        }
                                    ],
                                    data.execEnabled,
                                    data.execAllowed,
                                    data.logsAllowed
                                )}
                                selectedTabKey={props.tab}
                                onTabSelected={selected => appContext.navigation.goto('.', {tab: selected}, {replace: true})}
                            />
                        </React.Fragment>
                    )}
                </DataLoader>
            )}
            {isAppSelected && (
                <Tabs
                    navTransparent={true}
                    tabs={getApplicationTabs()}
                    selectedTabKey={tab}
                    onTabSelected={selected => appContext.navigation.goto('.', {tab: selected}, {replace: true})}
                />
            )}
        </div>
    );
};
