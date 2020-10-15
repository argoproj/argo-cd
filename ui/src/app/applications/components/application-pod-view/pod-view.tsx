import {Tooltip} from 'argo-ui';
import * as React from 'react';

import {ApplicationTree, Node, NodeStatus, Pod, PodPhase, ResourceNode, ResourceStat} from '../../../shared/models';
import {nodeKey} from '../utils';
import './pod-view.scss';

export class PodView extends React.Component<{tree: ApplicationTree; onPodClick: (fullName: string) => void}> {
    public render() {
        return (
            <React.Fragment>
                <div className='nodes-container'>
                    {(getNodesFromTree(this.props.tree) || []).map(node => (
                        <div className='node white-box' key={node.metadata.name}>
                            <div className='node__container node__container--header'>
                                <span>{(node.metadata.name || 'Unknown').toUpperCase()}</span>
                                <i className='fa fa-info-circle' style={{marginLeft: 'auto'}} />
                            </div>
                            <div className='node__container'>
                                <div className='node__container node__container--stats'>{(node.status.capacity || []).map(r => Stat(r))}</div>
                                <div className='node__pod-container node__container'>
                                    <div className='node__pod-container__pods'>
                                        {node.pods.map(pod => (
                                            <Tooltip content={pod.metadata.name} key={pod.metadata.name}>
                                                <div className={`node__pod node__pod--${pod.status.phase.toLowerCase()}`} onClick={() => this.props.onPodClick(pod.fullName)} />
                                            </Tooltip>
                                        ))}
                                    </div>
                                    <div className='node__label'>PODS</div>
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            </React.Fragment>
        );
    }
}

function getNodesFromTree(tree: ApplicationTree): Node[] {
    const nodes: {[key: string]: Node} = {};
    tree.nodes.forEach((d: ResourceNode) => {
        if (d.kind === 'Pod') {
            const p: Pod = {
                fullName: nodeKey(d),
                metadata: {name: d.name},
                spec: {nodeName: 'Unknown'},
                status: {phase: PodPhase.PodUnknown}
            } as Pod;
            d.info.forEach(i => {
                if (i.name === 'Status Reason') {
                    p.status.phase = (i.value as PodPhase) || PodPhase.PodUnknown;
                } else if (i.name === 'Node') {
                    p.spec.nodeName = i.value;
                }
            });
            if (nodes[p.spec.nodeName]) {
                nodes[p.spec.nodeName].pods.push(p);
            } else {
                nodes[p.spec.nodeName] = {metadata: {name: p.spec.nodeName}, status: {} as NodeStatus, pods: [p]};
            }
        }
    });
    return Object.values(nodes);
}

function Stat(stat: ResourceStat) {
    return (
        <div className='node__pod__stat node__container' key={stat.name}>
            <Tooltip content={`${stat.used} / ${stat.quantity} used`}>
                <div className='node__pod__stat__bar'>
                    <div className='node__pod__stat__bar--fill' style={{height: `${100 * (stat.used / stat.quantity)}%`}} />
                </div>
            </Tooltip>
            <div className='node__label'>{stat.name.slice(0, 3).toUpperCase()}</div>
        </div>
    );
}
