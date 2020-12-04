import {DataLoader, DropDownMenu, MenuItem, NotificationType, Tooltip} from 'argo-ui';
import * as React from 'react';
import * as PropTypes from 'prop-types';
import {Checkbox as ReactCheckbox} from 'react-form';
import {Application, ApplicationTree, InfoItem, Metric, InfraNode, Pod, PodGroup, PodGroupType, PodPhase, ResourceList, ResourceName} from '../../../shared/models';
import {PodViewPreferences, services, ViewPreferences} from '../../../shared/services';
import {ErrorNotification} from '../../../shared/components';
import {ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {nodeKey, PodHealthIcon} from '../utils';
import {AppContext} from '../../../shared/context';
import {ResourceIcon} from '../resource-icon';
import {HealthStatusIcon, ComparisonStatusIcon} from '../utils';
import './pod-view.scss';
import Moment from 'react-moment';

interface PodViewProps {
    tree: ApplicationTree;
    onPodClick: (fullName: string) => void;
    app: Application;
}
export class PodView extends React.Component<PodViewProps> {
    private loader: DataLoader;
    private get appContext(): AppContext {
        return this.context as AppContext;
    }
    public static contextTypes = {
        apis: PropTypes.object,
    };
    private menuItemsFor(modes: PodGroupType[], prefs: ViewPreferences): MenuItem[] {
        const podPrefs = prefs.appDetails.podView || ({} as PodViewPreferences);
        return modes.map((mode) => ({
            title: (
                <React.Fragment>
                    {podPrefs.sortMode === mode && <i className='fa fa-check' />} {labelForSortMode[mode]}{' '}
                </React.Fragment>
            ),
            action: () => {
                services.viewPreferences.updatePreferences({appDetails: {...prefs.appDetails, podView: {...podPrefs, sortMode: mode}}});
                this.loader.reload();
            },
        }));
    }
    public render() {
        return (
            <DataLoader load={() => services.viewPreferences.getPreferences()}>
                {(prefs) => {
                    const podPrefs = prefs.appDetails.podView || ({} as PodViewPreferences);
                    return (
                        <React.Fragment>
                            <div className='pod-view__settings'>
                                <div className='pod-view__settings__section'>
                                    GROUP BY:&nbsp;
                                    <DropDownMenu
                                        anchor={() => (
                                            <button className='argo-button argo-button--base-o'>
                                                {labelForSortMode[podPrefs.sortMode]}&nbsp;&nbsp;
                                                <i className='fa fa-chevron-circle-down' />
                                            </button>
                                        )}
                                        items={this.menuItemsFor(['node', 'parentResource', 'topLevelResource'], prefs)}
                                    />
                                </div>
                            </div>
                            <DataLoader
                                ref={(loader) => (this.loader = loader)}
                                load={async () => {
                                    return podPrefs.sortMode === 'node' ? services.applications.getNodes(this.props.app.metadata.name) : null;
                                }}>
                                {(nodes) => {
                                    return (
                                        <div className='nodes-container'>
                                            {this.processTree(podPrefs.sortMode, nodes).map((group) => (
                                                <div className='node white-box' key={group.name}>
                                                    <div className='node__container--header'>
                                                        <div style={{display: 'flex', alignItems: 'center'}}>
                                                            <ResourceIcon
                                                                kind={group.kind || 'Unknown'}
                                                                customStyle={{
                                                                    marginRight: '10px',
                                                                }}
                                                            />
                                                            <div style={{lineHeight: '15px', wordWrap: 'break-word'}}>
                                                                <b>{group.name || 'unknown'}</b>
                                                                {group.resourceStatus && (
                                                                    <div>
                                                                        {group.resourceStatus.health && <HealthStatusIcon state={group.resourceStatus.health} />}
                                                                        {group.resourceStatus.status && <ComparisonStatusIcon status={group.resourceStatus.status} />}
                                                                    </div>
                                                                )}
                                                            </div>
                                                        </div>
                                                        {group.type === 'node' ? (
                                                            <div className='node__info--large'>
                                                                {(group.info || []).map((item) => (
                                                                    <div>
                                                                        {item.name}: <div>{item.value}</div>
                                                                    </div>
                                                                ))}
                                                            </div>
                                                        ) : (
                                                            <div className='node__info'>
                                                                {group.createdAt ? (
                                                                    <div>
                                                                        <Moment fromNow={true} ago={true}>
                                                                            {group.createdAt}
                                                                        </Moment>
                                                                    </div>
                                                                ) : null}
                                                                {group.info.map((infoItem) => (
                                                                    <div>{infoItem.value}</div>
                                                                ))}
                                                            </div>
                                                        )}
                                                    </div>
                                                    <div className='node__container'>
                                                        {Object.keys((group as InfraNode).metrics || {}).length > 0 && (
                                                            <div className='node__container node__container--stats'>
                                                                {Object.keys((group as InfraNode).metrics || {}).map((r) =>
                                                                    Stat(r as ResourceName, (group as InfraNode).metrics[r as ResourceName])
                                                                )}
                                                            </div>
                                                        )}
                                                        <div className='node__pod-container node__container'>
                                                            <div className='node__pod-container__pods'>
                                                                {group.pods.map((pod) => (
                                                                    <DropDownMenu
                                                                        anchor={() => (
                                                                            <Tooltip
                                                                                content={
                                                                                    <div>
                                                                                        {pod.metadata.name}
                                                                                        <div>Phase: {pod.status.phase}</div>
                                                                                        <div>Health: {pod.health}</div>
                                                                                    </div>
                                                                                }
                                                                                key={pod.metadata.name}>
                                                                                <div className={`node__pod node__pod--${pod.health.toLowerCase()}`}>
                                                                                    <PodHealthIcon state={{status: pod.health, message: ''}} />
                                                                                </div>
                                                                            </Tooltip>
                                                                        )}
                                                                        items={[
                                                                            {
                                                                                title: (
                                                                                    <React.Fragment>
                                                                                        <i className='fa fa-info-circle' /> Info
                                                                                    </React.Fragment>
                                                                                ),
                                                                                action: () => this.props.onPodClick(pod.fullName),
                                                                            },
                                                                            {
                                                                                title: (
                                                                                    <React.Fragment>
                                                                                        <i className='fa fa-trash' /> Delete
                                                                                    </React.Fragment>
                                                                                ),
                                                                                action: async () => {
                                                                                    this.appContext.apis.popup.prompt(
                                                                                        'Delete pod',
                                                                                        () => (
                                                                                            <div>
                                                                                                <p>Are your sure you want to delete Pod '{pod.name}'?</p>
                                                                                                <div className='argo-form-row' style={{paddingLeft: '30px'}}>
                                                                                                    <ReactCheckbox id='force-delete-checkbox' field='force' />
                                                                                                    <label htmlFor='force-delete-checkbox'>Force delete</label>
                                                                                                </div>
                                                                                            </div>
                                                                                        ),
                                                                                        {
                                                                                            submit: async (vals, _, close) => {
                                                                                                try {
                                                                                                    await services.applications.deleteResource(
                                                                                                        this.props.app.metadata.name,
                                                                                                        pod,
                                                                                                        !!vals.force
                                                                                                    );
                                                                                                    this.loader.reload();
                                                                                                    close();
                                                                                                } catch (e) {
                                                                                                    this.appContext.apis.notifications.show({
                                                                                                        content: <ErrorNotification title='Unable to delete resource' e={e} />,
                                                                                                        type: NotificationType.Error,
                                                                                                    });
                                                                                                }
                                                                                            },
                                                                                        }
                                                                                    );
                                                                                },
                                                                            },
                                                                        ]}></DropDownMenu>
                                                                ))}
                                                            </div>
                                                            <div className='node__label'>PODS</div>
                                                        </div>
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    );
                                }}
                            </DataLoader>
                        </React.Fragment>
                    );
                }}
            </DataLoader>
        );
    }

    private processTree(sortMode: PodGroupType, initNodes: InfraNode[]): PodGroup[] {
        const tree = this.props.tree;
        if (!tree) {
            return [];
        }
        const groupRefs: {[key: string]: PodGroup} = {};
        const parentsFor: {[key: string]: PodGroup[]} = {};

        if (sortMode === 'node' && initNodes) {
            initNodes.forEach((infraNode) => {
                const nodeName = infraNode.metadata ? (infraNode.metadata.labels ? infraNode.metadata.labels['kubernetes.io/hostname'] : 'Unknown') : 'Unknown';
                groupRefs[nodeName] = {
                    ...infraNode,
                    type: 'node',
                    kind: 'node',
                    name: nodeName,
                    pods: [],
                    info: [
                        {name: 'Kernel Version', value: infraNode.status.nodeInfo.kernelVersion},
                        {name: 'Operating System', value: infraNode.status.nodeInfo.operatingSystem},
                        {name: 'Architecture', value: infraNode.status.nodeInfo.architecture},
                    ],
                };
            });
        }

        (tree.nodes || []).forEach((rnode: ResourceTreeNode) => {
            if (sortMode !== 'node') {
                parentsFor[rnode.uid] = rnode.parentRefs as PodGroup[];
                groupRefs[rnode.uid] = {
                    pods: [] as Pod[],
                    ...groupRefs[rnode.uid],
                    ...rnode,
                    info: (rnode.info || []).filter((i) => !i.name.includes('Resource.')),
                    createdAt: rnode.createdAt,
                    resourceStatus: {health: rnode.health, status: rnode.status},
                };
            }

            if (rnode.kind !== 'Pod') return;

            const p: Pod = {
                ...rnode,
                fullName: nodeKey(rnode),
                metadata: {name: rnode.name},
                spec: {nodeName: 'Unknown'},
                status: {phase: PodPhase.PodUnknown},
                health: rnode.health.status,
            } as Pod;

            // Get node name for Pod
            rnode.info.forEach((i) => {
                if (i.name === 'Node') {
                    p.spec.nodeName = i.value;
                }
            });

            if (sortMode === 'node') {
                if (groupRefs[p.spec.nodeName]) {
                    const curNode = groupRefs[p.spec.nodeName] as InfraNode;
                    curNode.metrics = mergeResourceLists(curNode.metrics, InfoToResourceList(rnode.info));
                    curNode.pods.push(p);
                }
            } else if (sortMode === 'parentResource') {
                rnode.parentRefs.forEach((parentRef) => {
                    if (!groupRefs[parentRef.uid]) {
                        groupRefs[parentRef.uid] = {
                            kind: parentRef.kind,
                            type: sortMode,
                            name: parentRef.name,
                            pods: [p],
                        };
                    } else {
                        groupRefs[parentRef.uid].pods.push(p);
                    }
                });
            } else if (sortMode === 'topLevelResource') {
                let cur = rnode.uid;
                let parents = parentsFor[rnode.uid];
                while ((parents || []).length > 0) {
                    cur = parents[0].uid;
                    parents = parentsFor[cur];
                }
                if (groupRefs[cur]) groupRefs[cur].pods.push(p);
            }
        });

        return Object.values(groupRefs)
            .sort((a, b) => (a.name > b.name ? 1 : a.name === b.name ? 0 : -1)) // sort by name
            .filter((i) => (i.pods || []).length > 0); // filter out groups with no pods
    }
}

function InfoToResourceList(items: InfoItem[]): ResourceList {
    const resources = {} as ResourceList;
    items
        .filter((item) => item.name.includes('Resource.'))
        .forEach((item) => {
            const name = item.name.replace(/Resource.|Limit|Req/gi, '').toLowerCase() as ResourceName;
            resources[name] = resources[name] || ({} as Metric);
            if (item.name.includes('Limit')) {
                resources[name].limit = parseInt(item.value, 10);
            } else {
                resources[name].request = parseInt(item.value, 10);
            }
        });
    return resources;
}

const labelForSortMode = {
    node: 'Node',
    parentResource: 'Parent Resource',
    topLevelResource: 'Top Level Resource',
};

function mergeResourceLists(a: ResourceList, b: ResourceList): ResourceList {
    if (!a || !b) {
        return !a ? b : a;
    }
    const res = a;
    (Object.keys(b) as ResourceName[]).forEach((key) => {
        res[key].limit += b[key].limit;
        res[key].request += b[key].request;
    });
    return res;
}

function Stat(name: ResourceName, metric: Metric) {
    return (
        <div className='node__pod__stat' key={name}>
            <Tooltip
                key={name}
                content={
                    <React.Fragment>
                        <div>{name}:</div>
                        <div>{`${metric.request} / ${metric.limit}`}</div>
                    </React.Fragment>
                }>
                <div className='node__pod__stat__bar'>
                    <div className='node__pod__stat__bar--fill' style={{height: `${100 * (metric.request / metric.limit)}%`}} />
                </div>
            </Tooltip>
            <div className='node__label'>{name.slice(0, 3).toUpperCase()}</div>
        </div>
    );
}
