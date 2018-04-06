import * as classNames from 'classnames';
import * as dagre from 'dagre';
import * as React from 'react';

import * as models from '../../../shared/models';
import { ComparisonStatusIcon } from '../utils';

require('./application-resources-tree.scss');

interface Line { x1: number; y1: number; x2: number; y2: number; }

const NODE_WIDTH = 182;
const NODE_HEIGHT = 52;

const ICONCLASS_BY_KIND = {
    application: 'argo-icon-application',
    deployment: 'argo-icon-deployment',
    pod: 'argo-icon-docker',
    service: 'argo-icon-hosts',
} as any;

function getGraphSize(nodes: dagre.Node[]): { width: number, height: number} {
    let width = 0;
    let height = 0;
    nodes.forEach((node) => {
        width = Math.max(node.x + node.width, width);
        height = Math.max(node.y + node.height, height);
    });
    return {width, height};
}

export const ApplicationResourcesTree = (props: {app: models.Application, selectedNodeFullName?: string, onNodeClick?: (fullName: string) => any}) => {
    const graph = new dagre.graphlib.Graph();
    graph.setGraph({ rankdir: 'LR' });
    graph.setDefaultEdgeLabel(() => ({}));
    graph.setNode(`${props.app.kind}:${props.app.metadata.name}`, { state: props.app, width: NODE_WIDTH, height: NODE_HEIGHT });

    function addChildren<T extends (models.ResourceNode | models.ResourceState) & { fullName: string, children: models.ResourceNode[] }>(node: T) {
        graph.setNode(node.fullName, Object.assign({}, node, { width: NODE_WIDTH, height: NODE_HEIGHT}));
        for (const child of (node.children || [])) {
            const fullName = `${child.state.kind}:${child.state.metadata.name}`;
            addChildren({...child, fullName});
            graph.setEdge(node.fullName, fullName);
        }
    }

    for (const node of (props.app.status.comparisonResult.resources || [])) {
        const state = node.liveState || node.targetState;
        addChildren({...node, children: node.childLiveResources, fullName: `${state.kind}:${state.metadata.name}`});
        graph.setEdge(`${props.app.kind}:${props.app.metadata.name}`, `${state.kind}:${state.metadata.name}`);
    }

    dagre.layout(graph);

    const edges: {from: string, to: string, lines: Line[]}[] = [];
    graph.edges().forEach((edgeInfo) => {
        const edge = graph.edge(edgeInfo);
        const lines: Line[] = [];
        if (edge.points.length > 1) {
            for (let i = 1; i < edge.points.length; i++) {
                lines.push({ x1: edge.points[i - 1].x, y1: edge.points[i - 1].y, x2: edge.points[i].x, y2: edge.points[i].y });
            }
        }
        edges.push({ from: edgeInfo.v, to: edgeInfo.w, lines });
    });
    const size = getGraphSize(graph.nodes().map((id) => graph.node(id)));
    return (
        <div className='application-resources-tree' style={{width: size.width + 10, height: size.height + 10}}>
            {graph.nodes().map((fullName) => {
                    const node = graph.node(fullName) as (models.ResourceNode | models.ResourceState) & dagre.Node;
                    let kubeState: models.State;
                    let comparisonStatus: models.ComparisonStatus = null;
                    if (node.liveState || node.targetState) {
                        const resourceState = node as models.ResourceState;
                        kubeState = resourceState.targetState || resourceState.liveState;
                        comparisonStatus = resourceState.status;
                    } else {
                        const resourceNode = node as models.ResourceNode;
                        kubeState = resourceNode.state;
                    }
                    const kindIcon = ICONCLASS_BY_KIND[kubeState.kind.toLocaleLowerCase()] || 'fa fa-gears';
                    return (
                        <div onClick={() => props.onNodeClick && props.onNodeClick(fullName)} key={fullName} className={classNames('application-resources-tree__node', {
                            active: fullName === props.selectedNodeFullName,
                        })} style={{left: node.x, top: node.y, width: node.width, height: node.height}}>
                            <span title={kubeState.kind} className={`application-resources-tree__node-kind-icon icon ${kindIcon}`}/>
                            <span className='application-resources-tree__node-title' title={kubeState.metadata.name}>{kubeState.metadata.name}</span>
                            <div className='application-resources-tree__node-status-icon'>
                                {comparisonStatus != null && <ComparisonStatusIcon status={comparisonStatus}/>}
                            </div>
                        </div>
                    );
                })}
                {edges.map((edge) => (
                    <div key={`${edge.from}-${edge.to}`} className='application-resources-tree__edge'>
                    {edge.lines.map((line, i) => {
                        const distance = Math.sqrt(Math.pow(line.x1 - line.x2, 2) + Math.pow(line.y1 - line.y2, 2));
                        const xMid = (line.x1 + line.x2) / 2;
                        const yMid = (line.y1 + line.y2) / 2;
                        const angle = Math.atan2(line.y1 - line.y2, line.x1 - line.x2) * 180 / Math.PI;
                        return (
                            <div className='application-resources-tree__line' key={i}
                                style={{ width: distance, left: xMid - (distance / 2), top: yMid, transform: `translate(100px, 35px) rotate(${angle}deg)`}} />
                        );
                    })}</div>
                ))}
        </div>
    );
};
