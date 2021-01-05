import {DataLoader, DropDown, DropDownMenu, MenuItem, NotificationType, Tooltip} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Checkbox as ReactCheckbox} from 'react-form';
import Moment from 'react-moment';
import {EmptyState, ErrorNotification} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import {Application, ApplicationTree, InfoItem, InfraNode, Metric, Pod, ResourceList, ResourceName, ResourceNode, ResourceStatus} from '../../../shared/models';
import {PodViewPreferences, services, ViewPreferences} from '../../../shared/services';
import {ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {nodeKey, PodHealthIcon} from '../utils';
import {ComparisonStatusIcon, HealthStatusIcon} from '../utils';
import './pod-view.scss';

interface PodViewProps {
    tree: ApplicationTree;
    onItemClick: (fullName: string) => void;
    app: Application;
    nodeMenu?: (node: ResourceNode) => React.ReactNode;
}

export type PodGroupType = 'topLevelResource' | 'parentResource' | 'node';

export interface PodGroup extends Partial<ResourceNode> {
    type: PodGroupType;
    pods: Pod[];
    info?: InfoItem[];
    metrics?: ResourceList;
    resourceStatus?: Partial<ResourceStatus>;
    renderMenu?: () => React.ReactNode;
    fullName?: string;
}

export class PodView extends React.Component<PodViewProps> {
    private get appContext(): AppContext {
        return this.context as AppContext;
    }
    public static contextTypes = {
        apis: PropTypes.object
    };
    private loader: DataLoader;
    public render() {
        return (
            <DataLoader load={() => services.viewPreferences.getPreferences()}>
                {prefs => {
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
                                ref={loader => (this.loader = loader)}
                                load={async () => {
                                    return podPrefs.sortMode === 'node' ? services.applications.getNodes(this.props.app.metadata.name) : null;
                                }}>
                                {nodes => {
                                    const groups = this.processTree(podPrefs.sortMode, nodes) || [];
                                    return groups.length > 0 ? (
                                        <div className='pod-view__nodes-container'>
                                            {groups.map(group => (
                                                <div className={`pod-view__node white-box ${group.kind === 'node' && 'pod-view__node--large'}`} key={group.name}>
                                                    <div
                                                        className='pod-view__node__container--header'
                                                        onClick={() => this.props.onItemClick(group.fullName)}
                                                        style={{cursor: 'pointer'}}>
                                                        <div style={{display: 'flex', alignItems: 'center'}}>
                                                            <div style={{marginRight: '10px'}}>
                                                                <ResourceIcon kind={group.kind || 'Unknown'} />
                                                                <br />
                                                                {<div style={{textAlign: 'center'}}>{ResourceLabel({kind: group.kind})}</div>}
                                                            </div>
                                                            <div style={{lineHeight: '15px'}}>
                                                                <b style={{wordWrap: 'break-word'}}>{group.name || 'Unknown'}</b>
                                                                {group.resourceStatus && (
                                                                    <div>
                                                                        {group.resourceStatus.health && <HealthStatusIcon state={group.resourceStatus.health} />}
                                                                        &nbsp;
                                                                        {group.resourceStatus.status && <ComparisonStatusIcon status={group.resourceStatus.status} />}
                                                                    </div>
                                                                )}
                                                            </div>
                                                            <div style={{marginLeft: 'auto'}}>
                                                                {group.renderMenu && (
                                                                    <DropDown
                                                                        isMenu={true}
                                                                        anchor={() => (
                                                                            <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                                                <i className='fa fa-ellipsis-v' />
                                                                            </button>
                                                                        )}>
                                                                        {() => group.renderMenu()}
                                                                    </DropDown>
                                                                )}
                                                            </div>
                                                        </div>
                                                        {group.type === 'node' ? (
                                                            <div className='pod-view__node__info--large'>
                                                                {(group.info || []).map(item => (
                                                                    <div key={item.name}>
                                                                        {item.name}: <div>{item.value}</div>
                                                                    </div>
                                                                ))}
                                                            </div>
                                                        ) : (
                                                            <div className='pod-view__node__info'>
                                                                {group.createdAt ? (
                                                                    <div>
                                                                        <Moment fromNow={true} ago={true}>
                                                                            {group.createdAt}
                                                                        </Moment>
                                                                    </div>
                                                                ) : null}
                                                                {group.info.map(infoItem => (
                                                                    <div key={infoItem.name}>{infoItem.value}</div>
                                                                ))}
                                                            </div>
                                                        )}
                                                    </div>
                                                    <div className='pod-view__node__container'>
                                                        {Object.keys((group as InfraNode).metrics || {}).length > 0 && (
                                                            <div className='pod-view__node__container pod-view__node__container--stats'>
                                                                {Object.keys((group as InfraNode).metrics || {}).map(r =>
                                                                    stat(r as ResourceName, (group as InfraNode).metrics[r as ResourceName])
                                                                )}
                                                            </div>
                                                        )}
                                                        <div className='pod-view__node__pod-container pod-view__node__container'>
                                                            <div className='pod-view__node__pod-container__pods'>
                                                                {group.pods.map(pod => (
                                                                    <DropDownMenu
                                                                        key={pod.uid}
                                                                        anchor={() => (
                                                                            <Tooltip
                                                                                content={
                                                                                    <div>
                                                                                        {pod.metadata.name}
                                                                                        <div>Health: {pod.health}</div>
                                                                                    </div>
                                                                                }
                                                                                key={pod.metadata.name}>
                                                                                <div className={`pod-view__node__pod pod-view__node__pod--${pod.health.toLowerCase()}`}>
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
                                                                                action: () => this.props.onItemClick(pod.fullName)
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
                                                                                                        type: NotificationType.Error
                                                                                                    });
                                                                                                }
                                                                                            }
                                                                                        }
                                                                                    );
                                                                                }
                                                                            }
                                                                        ]}
                                                                    />
                                                                ))}
                                                            </div>
                                                            <div className='pod-view__node__label'>PODS</div>
                                                        </div>
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    ) : (
                                        <EmptyState icon=' fa fa-th'>
                                            <h4>Your application has no pod groups</h4>
                                            <h5>Try switching to tree or list view</h5>
                                        </EmptyState>
                                    );
                                }}
                            </DataLoader>
                        </React.Fragment>
                    );
                }}
            </DataLoader>
        );
    }
    private menuItemsFor(modes: PodGroupType[], prefs: ViewPreferences): MenuItem[] {
        const podPrefs = prefs.appDetails.podView || ({} as PodViewPreferences);
        return modes.map(mode => ({
            title: (
                <React.Fragment>
                    {podPrefs.sortMode === mode && <i className='fa fa-check' />} {labelForSortMode[mode]}{' '}
                </React.Fragment>
            ),
            action: () => {
                services.viewPreferences.updatePreferences({appDetails: {...prefs.appDetails, podView: {...podPrefs, sortMode: mode}}});
                this.loader.reload();
            }
        }));
    }

    private processTree(sortMode: PodGroupType, initNodes: InfraNode[]): PodGroup[] {
        const tree = this.props.tree;
        if (!tree) {
            return [];
        }
        const groupRefs: {[key: string]: PodGroup} = {};
        const parentsFor: {[key: string]: PodGroup[]} = {};

        if (sortMode === 'node' && initNodes) {
            initNodes.forEach(infraNode => {
                const nodeName = infraNode.metadata ? infraNode.metadata.name || 'Unknown' : 'Unknown';
                groupRefs[nodeName] = {
                    ...infraNode,
                    type: 'node',
                    kind: 'node',
                    name: nodeName,
                    pods: [],
                    info: [
                        {name: 'Kernel Version', value: infraNode.status.nodeInfo.kernelVersion},
                        {name: 'Operating System', value: infraNode.status.nodeInfo.operatingSystem},
                        {name: 'Architecture', value: infraNode.status.nodeInfo.architecture}
                    ]
                };
            });
        }

        const statusByKey = new Map<string, ResourceStatus>();
        this.props.app.status.resources.forEach(res => statusByKey.set(nodeKey(res), res));
        (tree.nodes || []).forEach((rnode: ResourceTreeNode) => {
            if (sortMode !== 'node') {
                parentsFor[rnode.uid] = rnode.parentRefs as PodGroup[];
                const fullName = nodeKey(rnode);
                const status = statusByKey.get(fullName);
                if ((rnode.parentRefs || []).length === 0) {
                    rnode.root = rnode;
                }
                groupRefs[rnode.uid] = {
                    pods: [] as Pod[],
                    fullName,
                    ...groupRefs[rnode.uid],
                    ...rnode,
                    info: (rnode.info || []).filter(i => !i.name.includes('Resource.')),
                    createdAt: rnode.createdAt,
                    resourceStatus: {health: rnode.health, status: status ? status.status : null},
                    renderMenu: () => this.props.nodeMenu(rnode)
                };
            }

            if (rnode.kind !== 'Pod') {
                return;
            }

            const p: Pod = {
                ...rnode,
                fullName: nodeKey(rnode),
                metadata: {name: rnode.name},
                spec: {nodeName: 'Unknown'},
                health: rnode.health.status
            } as Pod;

            // Get node name for Pod
            rnode.info.forEach(i => {
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
                rnode.parentRefs.forEach(parentRef => {
                    if (!groupRefs[parentRef.uid]) {
                        groupRefs[parentRef.uid] = {
                            kind: parentRef.kind,
                            type: sortMode,
                            name: parentRef.name,
                            pods: [p]
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
                if (groupRefs[cur]) {
                    groupRefs[cur].pods.push(p);
                }
            }
        });

        return Object.values(groupRefs)
            .sort((a, b) => (a.name > b.name ? 1 : a.name === b.name ? 0 : -1)) // sort by name
            .filter(i => (i.pods || []).length > 0); // filter out groups with no pods
    }
}

function InfoToResourceList(items: InfoItem[]): ResourceList {
    const resources = {} as ResourceList;
    items
        .filter(item => item.name.includes('Resource.'))
        .forEach(item => {
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
    topLevelResource: 'Top Level Resource'
};

function mergeResourceLists(a: ResourceList, b: ResourceList): ResourceList {
    if (!a || !b) {
        return !a ? b : a;
    }
    const res = a;
    (Object.keys(b) as ResourceName[]).forEach(key => {
        res[key].limit += b[key].limit;
        res[key].request += b[key].request;
    });
    return res;
}

function stat(name: ResourceName, metric: Metric) {
    return (
        <div className='pod-view__node__pod__stat' key={name}>
            <Tooltip
                key={name}
                content={
                    <React.Fragment>
                        <div>{name}:</div>
                        <div>{`${metric.request} / ${metric.limit}`}</div>
                    </React.Fragment>
                }>
                <div className='pod-view__node__pod__stat__bar'>
                    <div className='pod-view__node__pod__stat__bar--fill' style={{height: `${100 * (metric.request / metric.limit)}%`}} />
                </div>
            </Tooltip>
            <div className='pod-view__node__label'>{name.slice(0, 3).toUpperCase()}</div>
        </div>
    );
}
