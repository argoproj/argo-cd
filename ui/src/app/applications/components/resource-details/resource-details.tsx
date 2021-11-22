import {DataLoader, Tab, Tabs} from 'argo-ui';
import {useData} from 'argo-ui/v2';
import * as React from 'react';
import {EventsList, YamlEditor} from '../../../shared/components';
import {Context} from '../../../shared/context';
import {Application, ApplicationTree, AppSourceType, Event, RepoAppDetails, ResourceNode, State, SyncStatuses} from '../../../shared/models';
import {services} from '../../../shared/services';
import {ExtensionComponentProps} from '../../../shared/services/extensions-service';
import {NodeInfo, SelectNode} from '../application-details/application-details';
import {ApplicationNodeInfo} from '../application-node-info/application-node-info';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {ApplicationResourceEvents} from '../application-resource-events/application-resource-events';
import {ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';
import {ApplicationSummary} from '../application-summary/application-summary';
import {PodsLogsViewer} from '../pod-logs-viewer/pod-logs-viewer';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import * as AppUtils from '../utils';
import './resource-details.scss';

const jsonMergePatch = require('json-merge-patch');

interface ResourceDetailsProps {
    selectedNode: ResourceNode;
    updateApp: (app: Application) => Promise<any>;
    application: Application;
    isAppSelected: boolean;
    tree: ApplicationTree;
    tab?: string;
}

export const ResourceDetails = (props: ResourceDetailsProps) => {
    const {selectedNode, updateApp, application, isAppSelected, tree} = {...props};
    const appContext = React.useContext(Context);
    const tab = new URLSearchParams(appContext.history.location.search).get('tab');
    const selectedNodeInfo = NodeInfo(new URLSearchParams(appContext.history.location.search).get('node'));
    const selectedNodeKey = selectedNodeInfo.key;

    const page = parseInt(new URLSearchParams(appContext.history.location.search).get('page'), 10) || 0;
    const untilTimes = (new URLSearchParams(appContext.history.location.search).get('untilTimes') || '').split(',') || [];

    const getResourceTabs = (node: ResourceNode, state: State, podState: State, events: Event[], ExtensionComponent: React.ComponentType<ExtensionComponentProps>, tabs: Tab[]) => {
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
                },
                {
                    offset: (podState.spec.containers || []).length,
                    title: 'INIT CONTAINERS',
                    containers: podState.spec.initContainers || []
                }
            ];
            tabs = tabs.concat([
                {
                    key: 'logs',
                    icon: 'fa fa-align-left',
                    title: 'LOGS',
                    content: (
                        <div className='application-details__tab-content-full-height'>
                            <div className='row'>
                                <div className='columns small-3 medium-2'>
                                    {containerGroups.map(group => (
                                        <div key={group.title} style={{marginBottom: '1em'}}>
                                            {group.containers.length > 0 && <p>{group.title}</p>}
                                            {group.containers.map((container: any, i: number) => (
                                                <div
                                                    className='application-details__container'
                                                    key={container.name}
                                                    onClick={() => SelectNode(selectedNodeKey, group.offset + i, 'logs', appContext)}>
                                                    {group.offset + i === selectedNodeInfo.container && <i className='fa fa-angle-right' />}
                                                    <span title={container.name}>{container.name}</span>
                                                </div>
                                            ))}
                                        </div>
                                    ))}
                                </div>
                                <div className='columns small-9 medium-10'>
                                    <PodsLogsViewer
                                        podName={(state.kind === 'Pod' && state.metadata.name) || ''}
                                        group={node.group}
                                        kind={node.kind}
                                        name={node.name}
                                        namespace={podState.metadata.namespace}
                                        applicationName={application.metadata.name}
                                        containerName={AppUtils.getContainerName(podState, selectedNodeInfo.container)}
                                        page={{number: page, untilTimes}}
                                        setPage={pageData => appContext.navigation.goto('.', {page: pageData.number, untilTimes: pageData.untilTimes.join(',')})}
                                    />
                                </div>
                            </div>
                        </div>
                    )
                }
            ]);
        }
        if (ExtensionComponent && state) {
            tabs.push({
                title: 'More',
                key: 'extension',
                content: (
                    <div>
                        <ExtensionComponent tree={tree} resource={state} />
                    </div>
                )
            });
        }
        return tabs;
    };

    const getApplicationTabs = () => {
        const tabs: Tab[] = [
            {
                title: 'SUMMARY',
                key: 'summary',
                content: <ApplicationSummary app={application} updateApp={app => updateApp(app)} />
            },
            {
                title: 'PARAMETERS',
                key: 'parameters',
                content: (
                    <DataLoader
                        key='appDetails'
                        input={application}
                        load={app =>
                            services.repos.appDetails(app.spec.source, app.metadata.name).catch(() => ({
                                type: 'Directory' as AppSourceType,
                                path: application.spec.source.path
                            }))
                        }>
                        {(details: RepoAppDetails) => <ApplicationParameters save={app => updateApp(app)} application={application} details={details} />}
                    </DataLoader>
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
                            return services.applications.updateSpec(application.metadata.name, jsonMergePatch.apply(spec, JSON.parse(patch)));
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
                            await services.applications.managedResources(application.metadata.name, {
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
            content: <ApplicationResourceEvents applicationName={application.metadata.name} />
        });

        return tabs;
    };

    const [extension, , error] = useData(
        async () => {
            if (selectedNode?.kind && selectedNode?.group) {
                return await services.extensions.loadResourceExtension(selectedNode?.group || '', selectedNode?.kind || '');
            }
        },
        null,
        null,
        [selectedNode]
    );

    return (
        <div style={{width: '100%', height: '100%'}}>
            {selectedNode && (
                <DataLoader
                    noLoaderOnInputChange={true}
                    input={selectedNode.resourceVersion}
                    load={async () => {
                        const managedResources = await services.applications.managedResources(application.metadata.name, {
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
                        const liveState = await services.applications.getResource(application.metadata.name, resQuery).catch(() => null);
                        const events =
                            (liveState &&
                                (await services.applications.resourceEvents(application.metadata.name, {
                                    name: liveState.metadata.name,
                                    namespace: liveState.metadata.namespace,
                                    uid: liveState.metadata.uid
                                }))) ||
                            [];
                        let podState: State;
                        if (selectedNode.kind === 'Pod') {
                            podState = liveState;
                        } else {
                            const childPod = AppUtils.findChildPod(selectedNode, tree);
                            if (childPod) {
                                podState = await services.applications.getResource(application.metadata.name, childPod).catch(() => null);
                            }
                        }

                        return {controlledState, liveState, events, podState};
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
                                    onClick={() => appContext.navigation.goto('.', {deploy: AppUtils.nodeKey(selectedNode)})}
                                    style={{marginLeft: 'auto', marginRight: '5px'}}
                                    className='argo-button argo-button--base'>
                                    <i className='fa fa-sync-alt' /> SYNC
                                </button>
                                <button onClick={() => AppUtils.deletePopup(appContext, selectedNode, application)} className='argo-button argo-button--base'>
                                    <i className='fa fa-trash' /> DELETE
                                </button>
                            </div>
                            <Tabs
                                navTransparent={true}
                                tabs={getResourceTabs(selectedNode, data.liveState, data.podState, data.events, error.state ? null : extension?.component, [
                                    {
                                        title: 'SUMMARY',
                                        icon: 'fa fa-file-alt',
                                        key: 'summary',
                                        content: <ApplicationNodeInfo application={application} live={data.liveState} controlled={data.controlledState} node={selectedNode} />
                                    }
                                ])}
                                selectedTabKey={props.tab}
                                onTabSelected={selected => appContext.navigation.goto('.', {tab: selected})}
                            />
                        </React.Fragment>
                    )}
                </DataLoader>
            )}
            {isAppSelected && (
                <Tabs navTransparent={true} tabs={getApplicationTabs()} selectedTabKey={tab} onTabSelected={selected => appContext.navigation.goto('.', {tab: selected})} />
            )}
        </div>
    );
};
