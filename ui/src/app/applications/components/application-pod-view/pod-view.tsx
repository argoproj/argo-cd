import {Checkbox, DataLoader, Tooltip} from 'argo-ui';
import * as React from 'react';

import {Application, ApplicationTree, InfoItem, Metric, Node, Pod, PodGroup, PodGroupType, PodPhase, ResourceList, ResourceName} from '../../../shared/models';
import {PodViewPreferences, services} from '../../../shared/services';
import {ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {nodeKey, PodHealthIcon, PodPhaseIcon} from '../utils';
import {GetNodes} from './pod-view-mock-service';
import './pod-view.scss';

interface PodViewProps {
    tree: ApplicationTree;
    onPodClick: (fullName: string) => void;
    app: Application;
}
export class PodView extends React.Component<PodViewProps, {demoMode: boolean}> {
    private loader: DataLoader;
    constructor(props: PodViewProps) {
        super(props);
        this.state = {
            demoMode: false,
        };
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
                                    <button
                                        className={`argo-button argo-button--${this.state.demoMode ? 'base' : 'base-o'}`}
                                        onClick={() => {
                                            this.setState({demoMode: !this.state.demoMode});
                                            this.loader.reload();
                                        }}>
                                        DEMO MODE
                                    </button>
                                </div>
                                <div className='pod-view__settings__section'>
                                    SORT BY:&nbsp;
                                    <span>
                                        <Checkbox
                                            checked={podPrefs.sortMode === 'node'}
                                            onChange={() =>
                                                services.viewPreferences.updatePreferences({appDetails: {...prefs.appDetails, podView: {...podPrefs, sortMode: 'node'}}})
                                            }
                                        />
                                        &nbsp;Node
                                    </span>
                                    &nbsp;
                                    <span>
                                        <Checkbox
                                            checked={podPrefs.sortMode === 'parentResource'}
                                            onChange={() =>
                                                services.viewPreferences.updatePreferences({appDetails: {...prefs.appDetails, podView: {...podPrefs, sortMode: 'parentResource'}}})
                                            }
                                        />
                                        &nbsp;Parent Resource
                                    </span>
                                    &nbsp;
                                    <span>
                                        <Checkbox
                                            checked={podPrefs.sortMode === 'topLevelResource'}
                                            onChange={() =>
                                                services.viewPreferences.updatePreferences({
                                                    appDetails: {...prefs.appDetails, podView: {...podPrefs, sortMode: 'topLevelResource'}},
                                                })
                                            }
                                        />
                                        &nbsp;Top Level Resource
                                    </span>
                                </div>
                            </div>
                            <DataLoader
                                ref={(loader) => (this.loader = loader)}
                                load={async () => {
                                    if (podPrefs.sortMode === 'node') {
                                        return this.state.demoMode ? GetNodes(5) : services.applications.getNodes(this.props.app.metadata.name);
                                    }
                                    return null;
                                }}>
                                {(nodes) => {
                                    return (
                                        <div className='nodes-container'>
                                            {(this.state.demoMode ? (nodes as PodGroup[]) : this.processTree(podPrefs.sortMode, nodes)).map((node) => (
                                                <div className='node white-box' key={node.name}>
                                                    <div className='node__container--header'>
                                                        <div>
                                                            <b>
                                                                <i className='fa fa-hdd' />
                                                                &nbsp;
                                                                {(node.name || 'Unknown').toUpperCase()}
                                                            </b>
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
                                                                            ).toLowerCase()}`}
                                                                            onClick={() => this.props.onPodClick(pod.fullName)}>
                                                                            {podPrefs.colorMode === 'phase' ? (
                                                                                <PodPhaseIcon state={pod.status.phase} />
                                                                            ) : (
                                                                                <PodHealthIcon state={{status: pod.health, message: ''}} />
                                                                            )}
                                                                        </div>
                                                                    </Tooltip>
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
            if (d.kind === 'Pod') {
                const p: Pod = {
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
                            g.info.push({name: 'Version', value: g.version || 'Unknown'});
                            groupRefs[ref.name] = g;
                        } else {
                            groupRefs[ref.name].pods.push(p);
                        }
                    });
                }
            }
        });
        return Object.values(groupRefs);
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
