import { DropDownMenu, MenuItem } from 'argo-ui';
import * as classNames from 'classnames';
import * as dagre from 'dagre';
import * as React from 'react';
import * as models from '../../../shared/models';
import { ComparisonStatusIcon, getStateAndNode, HealthStatusIcon } from '../utils';

require('./application-resources-tree.scss');

interface Line { x1: number; y1: number; x2: number; y2: number; }

const NODE_WIDTH = 282;
const NODE_HEIGHT = 52;

const ICON_CLASS_BY_KIND = {
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

// CountReadyContainerStatuses takes a list of container statuses and counts the running ones.
function countReadyContainerStatuses(containerStatuses: Map<string, any>): number {
    let total = 0;
    containerStatuses.forEach((containerStatus) => {
        if ('running' in containerStatus.state) {
            total++;
        }
    });
    return total;
}

function filterGraph(graph: dagre.graphlib.Graph, predicate: (node: models.ResourceNode | models.ResourceState) => boolean) {
    graph.nodes().forEach((nodeId) => {
        const node = graph.node(nodeId) as (models.ResourceNode | models.ResourceState) & dagre.Node;
        if (!predicate(node)) {
            const childIds = graph.successors(nodeId);
            const parentIds = graph.predecessors(nodeId);
            graph.removeNode(nodeId);
            childIds.forEach((childId: any) => {
                parentIds.forEach((parentId: any) => {
                    graph.setEdge(parentId, childId);
                });
            });
        }
    });
}

function sortNodes(nodes: models.ResourceNode[]): models.ResourceNode[] {
    return nodes.slice().sort(
        (first, second) => `${first.state.metadata.namespace}/${first.state.metadata.name}`.localeCompare(`${second.state.metadata.namespace}/${second.state.metadata.name}`));
}

export const ApplicationResourcesTree = (props: {
    app: models.Application,
    kindsFilter: string[],
    selectedNodeFullName?: string,
    onNodeClick?: (fullName: string) => any,
    nodeMenuItems?: (node: models.ResourceNode | models.ResourceState) => MenuItem[],
    nodeLabels?: (node: models.ResourceNode | models.ResourceState) => string[];
}) => {
    const graph = new dagre.graphlib.Graph();
    graph.setGraph({ rankdir: 'LR', marginx: -100 });
    graph.setDefaultEdgeLabel(() => ({}));
    graph.setNode(`${props.app.kind}:${props.app.metadata.name}`, { state: props.app, width: NODE_WIDTH, height: NODE_HEIGHT });

    function addChildren<T extends (models.ResourceNode | models.ResourceState) & { fullName: string, children: models.ResourceNode[] }>(node: T) {
        graph.setNode(node.fullName, Object.assign({}, node, { width: NODE_WIDTH, height: NODE_HEIGHT}));
        for (const child of sortNodes(node.children || [])) {
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

    if (props.kindsFilter.length > 0) {
        filterGraph(graph, (res) => {
            const {resourceNode} = getStateAndNode(res);
            return resourceNode.state.kind === 'Application' || props.kindsFilter.indexOf(resourceNode.state.kind) > -1;
        });
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
        <div className='application-resources-tree' style={{width: size.width + 150, height: size.height + 150}}>
            {graph.nodes().map((fullName) => {
                const node = graph.node(fullName) as (models.ResourceNode | models.ResourceState) & dagre.Node;
                let kubeState: models.State;
                let comparisonStatus: models.ComparisonStatus = null;
                let healthState: models.HealthStatus = null;
                if (node.liveState || node.targetState) {
                    const resourceState = node as models.ResourceState;
                    kubeState = resourceState.targetState || resourceState.liveState;
                    comparisonStatus = resourceState.status;
                    healthState = resourceState.health;
                } else {
                    const resourceNode = node as models.ResourceNode;
                    kubeState = resourceNode.state;
                    if (kubeState.kind === 'Application') {
                        const appState = kubeState as models.Application;
                        comparisonStatus = appState.status.comparisonResult.status;
                        healthState = appState.status.health;
                    }
                }
                const kindIcon = ICON_CLASS_BY_KIND[kubeState.kind.toLocaleLowerCase()] || 'fa fa-gears';
                return (
                    <div onClick={() => props.onNodeClick && props.onNodeClick(fullName)} key={fullName} className={classNames('application-resources-tree__node', {
                        active: fullName === props.selectedNodeFullName,
                    })} style={{left: node.x, top: node.y, width: node.width, height: node.height}}>
                        <div className={classNames('application-resources-tree__node-kind-icon', {
                            'application-resources-tree__node-kind-icon--big': kubeState.kind === 'Application',
                        })}>
                            <i title={kubeState.kind} className={`icon ${kindIcon}`}/>
                        </div>
                        <div className='application-resources-tree__node-content'>
                            <span className='application-resources-tree__node-title' title={kubeState.metadata.name}>{kubeState.metadata.name}</span>
                            <div className={classNames('application-resources-tree__node-status-icon', {
                                'application-resources-tree__node-status-icon--offset': kubeState.kind === 'Application',
                            })}>
                                {comparisonStatus != null && <ComparisonStatusIcon status={comparisonStatus}/>}
                                {healthState != null && <HealthStatusIcon state={healthState}/>}
                            </div>
                        </div>
                        <div className='application-resources-tree__node-labels'>
                            {props.nodeLabels && props.nodeLabels(node).map((label) => <span key={label}>{label}</span>)}
                            <span>{kubeState.kind}</span>
                            {kubeState.status && kubeState.status.containerStatuses && (
                                <span>{countReadyContainerStatuses(kubeState.status.containerStatuses)}/{kubeState.status.containerStatuses.length} containers ready</span>
                            )}
                        </div>
                        {props.nodeMenuItems && (
                            <div className='application-resources-tree__node-menu'>
                                <DropDownMenu anchor={() => <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                    <i className='fa fa-ellipsis-v'/>
                                </button>} items={props.nodeMenuItems(node)}/>
                            </div>
                        )}
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
                            style={{ width: distance, left: xMid - (distance / 2), top: yMid, transform: `translate(150px, 35px) rotate(${angle}deg)`}} />
                    );
                })}</div>
            ))}
        </div>
    );
};
