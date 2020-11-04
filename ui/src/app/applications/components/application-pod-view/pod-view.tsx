import {Tooltip, DataLoader, Checkbox} from 'argo-ui';
import * as React from 'react';

import {ApplicationTree, Application, Node, Pod, PodPhase, ResourceName} from '../../../shared/models';
import {ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {nodeKey, HealthStatusIcon, PodPhaseIcon} from '../utils';
import './pod-view.scss';
import {services} from '../../../shared/services';
import {GetNodes} from './pod-view-mock-service';

interface PodViewProps {
    tree: ApplicationTree;
    onPodClick: (fullName: string) => void;
    app: Application;
}

interface PodViewState {
    podColorMode: ColorMode;
    demoMode: boolean;
}

enum ColorMode {
    Health,
    Phase
}

export class PodView extends React.Component<PodViewProps, PodViewState> {
    private loader: DataLoader;
    constructor(props: PodViewProps) {
        super(props);
        this.state = {
            podColorMode: ColorMode.Health,
            demoMode: false
        };
    }
    public render() {
        return (
            <React.Fragment>
                <div className='pod-view__settings'>
                    <div className='pod-view__settings__section'>
                        DISPLAY:&nbsp;
                        <span>
                            <Checkbox checked={!!this.state.podColorMode} onChange={() => this.setState({podColorMode: ColorMode.Phase})} />
                            &nbsp;Phase
                        </span>
                        &nbsp;
                        <span>
                            <Checkbox checked={!this.state.podColorMode} onChange={() => this.setState({podColorMode: ColorMode.Health})} />
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
                </div>
                <DataLoader
                    ref={loader => (this.loader = loader)}
                    load={async () => {
                        return this.state.demoMode ? GetNodes(5) : await services.applications.getNodes(this.props.app.metadata.name);
                    }}>
                    {nodes => (
                        <div className='nodes-container'>
                            {(this.state.demoMode ? nodes : populateNodesFromTree(nodes || [], this.props.tree)).map(node => (
                                <div className='node white-box' key={node.metadata.name}>
                                    <div className='node__container--header'>
                                        <div>{(node.metadata.labels['kubernetes.io/hostname'] || 'Unknown').toUpperCase()}</div>
                                        <div>
                                            {node.status.nodeInfo.kernelVersion}, {node.status.nodeInfo.operatingSystem}, {node.status.nodeInfo.architecture}
                                        </div>
                                    </div>
                                    <div className='node__container'>
                                        <div className='node__container node__container--stats'>
                                            {(Object.keys(node.status.capacity || {}) || []).map(r =>
                                                Stat(r as ResourceName, node.status.capacity[r as ResourceName], node.status.allocatable[r as ResourceName])
                                            )}
                                        </div>
                                        <div className='node__pod-container node__container'>
                                            <div className='node__pod-container__pods'>
                                                {node.pods.map(pod => (
                                                    <Tooltip
                                                        content={
                                                            <div>
                                                                {pod.metadata.name}, {pod.status.phase}
                                                            </div>
                                                        }
                                                        key={pod.metadata.name}>
                                                        <div
                                                            className={`node__pod node__pod--${(this.state.podColorMode ? pod.status.phase.replace('Pod', '') : pod.health).toLowerCase()}`}
                                                            onClick={() => this.props.onPodClick(pod.fullName)}>
                                                            {this.state.podColorMode ? (
                                                                <PodPhaseIcon state={pod.status.phase} />
                                                            ) : (
                                                                <HealthStatusIcon state={{status: pod.health, message: ''}} />
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
                    )}
                </DataLoader>
            </React.Fragment>
        );
    }
}

function populateNodesFromTree(nodes: Node[], tree: ApplicationTree): Node[] {
    if (!tree) {
        return [];
    }
    const nodeRefs: {[key: string]: Node} = {};
    (nodes || []).forEach((n: Node) => {
        const hostname = n.metadata.labels['kubernetes.io/hostname'];
        n.pods = [];
        nodeRefs[hostname] = n;
    });
    (tree.nodes || []).forEach((d: ResourceTreeNode) => {
        if (d.kind === 'Pod') {
            const p: Pod = {
                fullName: nodeKey(d),
                metadata: {name: d.name},
                spec: {nodeName: 'Unknown'},
                status: {phase: PodPhase.PodUnknown},
                health: d.health.status
            } as Pod;
            d.info.forEach(i => {
                if (i.name === 'Status Reason') {
                    p.status.phase = (i.value as PodPhase) || PodPhase.PodUnknown;
                } else if (i.name === 'Node') {
                    p.spec.nodeName = i.value;
                }
            });
            if (nodeRefs[p.spec.nodeName]) {
                nodeRefs[p.spec.nodeName].pods.push(p);
            }
        }
    });
    return Object.values(nodeRefs);
}

function Stat(name: ResourceName, capacity: number, allocatable: number) {
    return (
        <div className='node__pod__stat node__container' key={name}>
            <Tooltip
                key={name}
                content={
                    <React.Fragment>
                        <div>{name}:</div>
                        <div>{`${allocatable} / ${capacity} available`}</div>
                    </React.Fragment>
                }>
                <div className='node__pod__stat__bar'>
                    <div className='node__pod__stat__bar--fill' style={{height: `${100 * ((capacity - allocatable) / capacity)}%`}} />
                </div>
            </Tooltip>
            <div className='node__label'>{name.slice(0, 3).toUpperCase()}</div>
        </div>
    );
}
