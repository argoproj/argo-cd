import {Application, ApplicationTree, AppSourceType, Event, RepoAppDetails, ResourceNode, State} from '../../../shared/models';
import * as React from 'react';
import {services} from '../../../shared/services';
import * as AppUtils from '../utils';
import {DataLoader, Tab, Tabs} from 'argo-ui';
import {ApplicationSummary} from '../application-summary/application-summary';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {EventsList, YamlEditor} from '../../../shared/components';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';
import {ApplicationResourceEvents} from '../application-resource-events/application-resource-events';
import {Context} from '../../../shared/context';
import {PodsLogsViewer} from '../pod-logs-viewer/pod-logs-viewer';

const jsonMergePatch = require('json-merge-patch');

interface ResourceDetailsProps {
    selectedNode: ResourceNode;
    updateApp: (app: Application) => Promise<any>;
    application: Application;
    isAppSelected: boolean;
    tree: ApplicationTree;
    tabs: (data: any) => Tab[];
}

const getResourceTabs = (application: Application, node: ResourceNode, state: State, podState: State, events: Event[], tabs: Tab[]) => {
    if (state) {
        const numErrors = events.filter(event => event.type !== 'Normal').reduce((total, event) => total + event.count, 0);
        tabs.push({
            title: 'EVENTS',
            badge: (numErrors > 0 && numErrors) || null,
            key: 'events',
            content: (
                <div className='application-resource-events'>
                    <EventsList events={events} />
                </div>
            )
        });
    }
    if (podState) {
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
                                                onClick={() => this.selectNode(this.selectedNodeKey, group.offset + i, 'logs')}>
                                                {group.offset + i === this.selectedNodeInfo.container && <i className='fa fa-angle-right' />}
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
                                    containerName={AppUtils.getContainerName(podState, this.selectedNodeInfo.container)}
                                />
                            </div>
                        </div>
                    </div>
                )
            }
        ]);
    }
    return tabs;
};

export const ResourceDetails = (props: ResourceDetailsProps) => {
    const {selectedNode, updateApp, application, isAppSelected, tree, tabs} = {...props};
    const appContext = React.useContext(Context);
    const tab = new URLSearchParams(appContext.history.location.search).get('tab');

    return (
        <div style={{width: '100%', height: '100%', backgroundColor: 'white'}}>
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
                    {data => <Tabs navTransparent={true} tabs={tabs(data)} selectedTabKey={tab} onTabSelected={selected => appContext.navigation.goto('.', {tab: selected})} />}
                </DataLoader>
            )}
            {isAppSelected && (
                <Tabs
                    navTransparent={true}
                    tabs={[
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
                                    input={application.spec.source}
                                    load={src =>
                                        services.repos.appDetails(src).catch(() => ({
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
                        },
                        {
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
                        },
                        {
                            title: 'EVENTS',
                            key: 'event',
                            content: <ApplicationResourceEvents applicationName={application.metadata.name} />
                        }
                    ]}
                    selectedTabKey={tab}
                    onTabSelected={selected => appContext.navigation.goto('.', {tab: selected})}
                />
            )}
        </div>
    );
};
