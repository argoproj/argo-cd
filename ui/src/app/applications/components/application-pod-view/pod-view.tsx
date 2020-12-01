import {Checkbox, DataLoader, DropDownMenu, NotificationType, Tooltip} from 'argo-ui';
import * as React from 'react';
import * as PropTypes from 'prop-types';
import {Checkbox as ReactCheckbox} from 'react-form';
import {Application, ApplicationTree, InfoItem, Metric, Node, Pod, PodGroup, PodGroupType, PodPhase, ResourceList, ResourceName} from '../../../shared/models';
import {PodViewPreferences, services} from '../../../shared/services';
import {ErrorNotification} from '../../../shared/components';
import {ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {nodeKey, PodHealthIcon, PodPhaseIcon} from '../utils';
import {AppContext} from '../../../shared/context';
import {ResourceIcon} from '../resource-icon';

import './pod-view.scss';

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
    public render() {
        return (
            <DataLoader load={() => services.viewPreferences.getPreferences()}>
                {(prefs) => {
                    const podPrefs = prefs.appDetails.podView || ({} as PodViewPreferences);
                    return (
                        <React.Fragment>
                            <div className='pod-view__settings'>
                                <div className='pod-view__settings__section'>
                                    DISPLAY:&nbsp;
                                    <span>
                                        <Checkbox
                                            checked={podPrefs.colorMode === 'phase'}
                                            onChange={() =>
                                                services.viewPreferences.updatePreferences({appDetails: {...prefs.appDetails, podView: {...podPrefs, colorMode: 'phase'}}})
                                            }
                                        />
                                        &nbsp;Phase
                                    </span>
                                    &nbsp;
                                    <span>
                                        <Checkbox
                                            checked={podPrefs.colorMode === 'health'}
                                            onChange={() =>
                                                services.viewPreferences.updatePreferences({appDetails: {...prefs.appDetails, podView: {...podPrefs, colorMode: 'health'}}})
                                            }
                                        />
                                        &nbsp;Health
                                    </span>
                                </div>
                                <div className='pod-view__settings__section'>
                                    SORT BY:&nbsp;
                                    <DropDownMenu
                                        anchor={() => (
                                            <button className='argo-button argo-button--base-o'>
                                                {labelForSortMode(podPrefs.sortMode)}&nbsp;&nbsp;
                                                <i className='fa fa-chevron-circle-down' />
                                            </button>
                                        )}
                                        items={[
                                            {
                                                title: <React.Fragment>{podPrefs.sortMode === 'node' && <i className='fa fa-check' />} Node</React.Fragment>,
                                                action: () =>
                                                    services.viewPreferences.updatePreferences({appDetails: {...prefs.appDetails, podView: {...podPrefs, sortMode: 'node'}}}),
                                            },
                                            {
                                                title: <React.Fragment>{podPrefs.sortMode === 'parentResource' && <i className='fa fa-check' />} Parent Resource</React.Fragment>,
                                                action: () =>
                                                    services.viewPreferences.updatePreferences({
                                                        appDetails: {...prefs.appDetails, podView: {...podPrefs, sortMode: 'parentResource'}},
                                                    }),
                                            },
                                            {
                                                title: (
                                                    <React.Fragment>{podPrefs.sortMode === 'topLevelResource' && <i className='fa fa-check' />} Top Level Resource</React.Fragment>
                                                ),
                                                action: () =>
                                                    services.viewPreferences.updatePreferences({
                                                        appDetails: {...prefs.appDetails, podView: {...podPrefs, sortMode: 'topLevelResource'}},
                                                    }),
                                            },
                                        ]}></DropDownMenu>
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
                                            {this.processTree(podPrefs.sortMode, nodes).map((node) => (
                                                <div className='node white-box' key={node.name}>
                                                    <div className='node__container--header'>
                                                        <div style={{display: 'flex', alignItems: 'center'}}>
                                                            <b>
                                                                {iconForGroupType(podPrefs.sortMode)}
                                                                &nbsp;
                                                                {(node.name || 'Unknown').toUpperCase()}
                                                            </b>
                                                            {podPrefs.sortMode !== 'node' && (
                                                                <ResourceIcon
                                                                    kind={node.kind}
                                                                    customStyle={{
                                                                        marginLeft: 'auto',
                                                                        width: '25px',
                                                                        height: '25px',
                                                                    }}
                                                                />
                                                            )}
                                                        </div>
                                                        {Info(node.info)}
                                                    </div>
                                                    <div className='node__container'>
                                                        {Object.keys((node as Node).metrics || {}).length > 0 && (
                                                            <div className='node__container node__container--stats'>
                                                                {Object.keys((node as Node).metrics || {}).map((r) =>
                                                                    Stat(r as ResourceName, (node as Node).metrics[r as ResourceName])
                                                                )}
                                                            </div>
                                                        )}
                                                        <div className='node__pod-container node__container'>
                                                            <div className='node__pod-container__pods'>
                                                                {node.pods.map((pod) => (
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
                                                                                <div
                                                                                    className={`node__pod node__pod--${(podPrefs.colorMode === 'phase'
                                                                                        ? (pod.status.phase || '').replace('Pod', '')
                                                                                        : pod.health
                                                                                    ).toLowerCase()}`}>
                                                                                    {podPrefs.colorMode === 'phase' ? (
                                                                                        <PodPhaseIcon state={pod.status.phase} />
                                                                                    ) : (
                                                                                        <PodHealthIcon state={{status: pod.health, message: ''}} />
                                                                                    )}
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

    private processTree(sortMode: PodGroupType, nodes: Node[]): PodGroup[] {
        const tree = this.props.tree;
        if (!tree) {
            return [];
        }
        const groupRefs: {[key: string]: PodGroup} = {};
        const parentsFor: {[key: string]: PodGroup[]} = {};

        if (sortMode === 'node' && nodes) {
            nodes.forEach((node) => {
                const name = node.metadata ? (node.metadata.labels ? node.metadata.labels['kubernetes.io/hostname'] : 'Unknown') : 'Unknown';
                const n = node as PodGroup;
                n.type = 'node';
                n.name = name;
                n.pods = [];
                n.info = [];
                n.info.push({name: 'Kernel Version', value: node.status.nodeInfo.kernelVersion});
                n.info.push({name: 'Operating System', value: node.status.nodeInfo.operatingSystem});
                n.info.push({name: 'Architecture', value: node.status.nodeInfo.architecture});
                groupRefs[name] = n;
            });
        }

        (tree.nodes || []).forEach((d: ResourceTreeNode) => {
            if (sortMode === 'topLevelResource') {
                parentsFor[d.name] = d.parentRefs as PodGroup[];
                if ((d.parentRefs || []).length === 0) {
                    const g = {} as PodGroup;
                    g.name = d.name;
                    g.info = [];
                    g.kind = d.kind;
                    g.info.push({name: 'Kind', value: d.kind || 'Unknown'});
                    g.info.push({name: 'Namespace', value: d.namespace || 'Unknown'});
                    g.info.push({name: 'Version', value: d.version || 'Unknown'});
                    g.pods = [];
                    groupRefs[d.name] = g;
                }
            }

            if (d.kind === 'Pod') {
                const p: Pod = {
                    ...d,
                    fullName: nodeKey(d),
                    metadata: {name: d.name},
                    spec: {nodeName: 'Unknown'},
                    status: {phase: PodPhase.PodUnknown},
                    health: d.health.status,
                } as Pod;
                d.info.forEach((i) => {
                    if (i.name === 'Status Reason') {
                        p.status.phase = (i.value as PodPhase) || PodPhase.PodUnknown;
                    } else if (i.name === 'Node') {
                        p.spec.nodeName = i.value;
                    }
                });
                if (sortMode === 'node') {
                    if (groupRefs[p.spec.nodeName]) {
                        const curNode = groupRefs[p.spec.nodeName] as Node;
                        curNode.metrics = mergeResourceLists(curNode.metrics, InfoToResourceList(d.info));
                        curNode.pods.push(p);
                    } else {
                        groupRefs[p.spec.nodeName] = {
                            name: p.spec.nodeName,
                            type: 'node',
                            metrics: InfoToResourceList(d.info),
                            pods: [p],
                        };
                    }
                } else if (sortMode === 'parentResource') {
                    d.parentRefs.forEach((ref) => {
                        if (!groupRefs[ref.name]) {
                            const g = ref as PodGroup;
                            g.pods = [];
                            g.pods.push(p);
                            g.info = [];
                            g.info.push({name: 'Kind', value: g.kind || 'Unknown'});
                            g.info.push({name: 'Namespace', value: g.namespace || 'Unknown'});
                            groupRefs[ref.name] = g;
                        } else {
                            groupRefs[ref.name].pods.push(p);
                        }
                    });
                } else if (sortMode === 'topLevelResource') {
                    let cur = d.name;
                    let parents = parentsFor[d.name];
                    while ((parents || []).length > 0) {
                        cur = parents[0].name;
                        parents = parentsFor[cur];
                    }
                    groupRefs[cur].pods.push(p);
                }
            }
        });
        return Object.values(groupRefs)
            .sort((a, b) => (a.name > b.name ? 1 : a.name === b.name ? 0 : -1))
            .filter((i) => (i.pods || []).length > 0);
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

function labelForSortMode(mode: PodGroupType) {
    switch (mode) {
        case 'node':
            return 'Node';
        case 'parentResource':
            return 'Parent Resource';
        case 'topLevelResource':
            return 'Top Level Resource';
    }
}

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

function iconForGroupType(type: PodGroupType) {
    switch (type) {
        case 'node':
            return <i className='fa fa-hdd' />;
        default:
            return <i className='fa fa-code-branch' />;
    }
}

function Info(items: InfoItem[]) {
    if (!items) {
        return null;
    }
    return (
        <div className='node__info'>
            {items.map((item) => (
                <div>
                    {item.name}: <div>{item.value}</div>
                </div>
            ))}
        </div>
    );
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
