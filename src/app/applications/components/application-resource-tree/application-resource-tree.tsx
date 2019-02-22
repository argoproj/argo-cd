import { DropDownMenu, MenuItem } from 'argo-ui';
import * as classNames from 'classnames';
import * as dagre from 'dagre';
import * as React from 'react';
import * as models from '../../../shared/models';
import { ComparisonStatusIcon, HealthStatusIcon, ICON_CLASS_BY_KIND, isAppNode, nodeKey, ResourceTreeNode } from '../utils';

require('./application-resource-tree.scss');

interface Line { x1: number; y1: number; x2: number; y2: number; }

const NODE_WIDTH = 282;
const NODE_HEIGHT = 52;
const FILTERED_INDICATOR = '__filtered_indicator__';

function getGraphSize(nodes: dagre.Node[]): { width: number, height: number} {
    let width = 0;
    let height = 0;
    nodes.forEach((node) => {
        width = Math.max(node.x + node.width, width);
        height = Math.max(node.y + node.height, height);
    });
    return {width, height};
}

function filterGraph(app: models.Application, graph: dagre.graphlib.Graph, predicate: (node: ResourceTreeNode) => boolean) {
    let filtered = 0;
    graph.nodes().forEach((nodeId) => {
        const node: ResourceTreeNode = graph.node(nodeId) as any;
        const parentIds = graph.predecessors(nodeId);
        if (node.root != null && !predicate(node)) {
            const childIds = graph.successors(nodeId);
            graph.removeNode(nodeId);
            filtered++;
            childIds.forEach((childId: any) => {
                parentIds.forEach((parentId: any) => {
                    graph.setEdge(parentId, childId);
                });
            });
        }
    });
    if (filtered) {
        graph.setNode(FILTERED_INDICATOR, { height: NODE_HEIGHT, width: NODE_WIDTH, count: filtered });
        graph.setEdge(appNodeKey(app), FILTERED_INDICATOR);
    }
}

function compareNodes(first: models.ResourceNode, second: models.ResourceNode) {
    return nodeKey(first).localeCompare(nodeKey(second));
}

function appNodeKey(app: models.Application) {
    return nodeKey({group: 'argoproj.io', kind: app.kind, name: app.metadata.name, namespace: app.metadata.namespace });
}

class NodeUpdateAnimation extends React.PureComponent<{ resourceVersion: string; }, {  ready: boolean }> {
    constructor(props: { resourceVersion: string; }) {
        super(props);
        this.state = { ready: false };
    }

    public render() {
        return this.state.ready && <div key={this.props.resourceVersion} className='application-resource-tree__node-animation'/>;
    }

    public componentDidUpdate(prevProps: { resourceVersion: string; }) {
        if (prevProps.resourceVersion && this.props.resourceVersion !== prevProps.resourceVersion) {
            this.setState({ ready: true });
        }
    }
}

function filteredNode(fullName: string, node: { count: number } & dagre.Node, onClearFilter: () => any) {
    const indicators = new Array<number>();
    let count = Math.min(node.count - 1, 3);
    while (count > 0) {
        indicators.push(count--);
    }
    return (
        <React.Fragment key={fullName}>
            <div className='application-resource-tree__node' style={{left: node.x, top: node.y, width: node.width, height: node.height}}>
                <div className='application-resource-tree__node-kind-icon '>
                    <i className='icon fa fa-filter'/>
                </div>
                <div className='application-resource-tree__node-content'>
                    <a className='application-resource-tree__node-title' onClick={onClearFilter}>show {node.count} hidden resource{node.count > 1 && 's'}</a>
                </div>
            </div>
            {indicators.map((i) => (
                <div key={i} className='application-resource-tree__node application-resource-tree__filtered-indicator'
                    style={{left: node.x + i * 2, top: node.y +  i * 2, width: node.width, height: node.height}}/>
            ))}
        </React.Fragment>
    );
}

export const ApplicationResourceTree = (props: {
    app: models.Application,
    resources: ResourceTreeNode[],
    nodeFilter: (node: ResourceTreeNode) => boolean,
    selectedNodeFullName?: string,
    onNodeClick?: (fullName: string) => any,
    nodeMenuItems?: (node: models.ResourceNode) => MenuItem[],
    onClearFilter: () => any;
}) => {
    const graph = new dagre.graphlib.Graph();
    graph.setGraph({ rankdir: 'LR', marginx: -100 });
    graph.setDefaultEdgeLabel(() => ({}));
    const appNode = {
        kind: props.app.kind,
        name: props.app.metadata.name,
        namespace: props.app.metadata.namespace,
        resourceVersion: props.app.metadata.resourceVersion,
        group: 'argoproj.io',
        version: '',
        children: Array(),
        status: props.app.status.sync.status,
        health: props.app.status.health,
        info: (props.app.spec.source.componentParameterOverrides || []).length > 0 ? [{
            name: 'Parameter overrides',
            value: `${props.app.spec.source.componentParameterOverrides.length} parameter override(s)`,
        }] : [],
    };
    graph.setNode(appNodeKey(props.app), { ...appNode, width: NODE_WIDTH, height: NODE_HEIGHT });

    function addChildren<T extends (models.ResourceNode | models.ResourceDiff) & { key: string, children: models.ResourceNode[] }>(node: T, root: ResourceTreeNode) {
        graph.setNode(node.key, Object.assign({}, node, { width: NODE_WIDTH, height: NODE_HEIGHT}));
        for (const child of (node.children || []).slice().sort(compareNodes)) {
            const key = nodeKey(child);
            addChildren({...child, key, root}, root);
            graph.setEdge(node.key, key);
        }
    }

    const resources = (props.resources || []).slice().sort(compareNodes);
    for (const res of resources) {
        addChildren({...res, root: res, children: res.children, key: nodeKey(res)}, res);
        graph.setEdge(appNodeKey(props.app), nodeKey(res));
    }

    if (props.nodeFilter) {
        filterGraph(props.app, graph, props.nodeFilter);
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
        <div className='application-resource-tree' style={{width: size.width + 150, height: size.height + 250}}>
            {graph.nodes().map((fullName) => {
                if (fullName === FILTERED_INDICATOR) {
                    return filteredNode(fullName, graph.node(fullName) as any, props.onClearFilter);
                }
                const node = graph.node(fullName) as (ResourceTreeNode) & dagre.Node;
                let comparisonStatus: models.SyncStatusCode = null;
                let healthState: models.HealthStatus = null;
                if (node.status || node.health) {
                    comparisonStatus = node.status;
                    healthState = node.health;
                }
                const kindIcon = ICON_CLASS_BY_KIND[node.kind.toLocaleLowerCase()] || 'fa fa-gears';
                return (
                    <div onClick={() => props.onNodeClick && props.onNodeClick(fullName)} key={fullName} className={classNames('application-resource-tree__node', {
                        active: fullName === props.selectedNodeFullName,
                    })} style={{left: node.x, top: node.y, width: node.width, height: node.height}}>
                        {!isAppNode(node) && <NodeUpdateAnimation resourceVersion={node.resourceVersion} />}
                        <div className={classNames('application-resource-tree__node-kind-icon', {
                            'application-resource-tree__node-kind-icon--big': isAppNode(node),
                        })}>
                            <i title={node.kind} className={`icon ${kindIcon}`}/>
                        </div>
                        <div className='application-resource-tree__node-content'>
                            <span className='application-resource-tree__node-title' title={node.name}>{node.name}</span>
                            <div className={classNames('application-resource-tree__node-status-icon', {
                                'application-resource-tree__node-status-icon--offset': isAppNode(node),
                            })}>
                                {comparisonStatus != null && <ComparisonStatusIcon status={comparisonStatus}/>}
                                {node.hook && (<i title='Resource lifecycle hook' className='fa fa-anchor' />)}
                                {healthState != null && <HealthStatusIcon state={healthState}/>}
                            </div>
                        </div>
                        <div className='application-resource-tree__node-labels'>
                            {(node.info || []).map((tag, i) => <span title={`${tag.name}:${tag.value}`} key={i}>{tag.value}</span>)}
                            <span>{node.kind}</span>
                        </div>
                        {props.nodeMenuItems && (
                            <div className='application-resource-tree__node-menu'>
                                <DropDownMenu anchor={() => <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                    <i className='fa fa-ellipsis-v'/>
                                </button>} items={props.nodeMenuItems(node)}/>
                            </div>
                        )}
                    </div>
                );
            })}
            {edges.map((edge) => (
                <div key={`${edge.from}-${edge.to}`} className='application-resource-tree__edge'>
                {edge.lines.map((line, i) => {
                    const distance = Math.sqrt(Math.pow(line.x1 - line.x2, 2) + Math.pow(line.y1 - line.y2, 2));
                    const xMid = (line.x1 + line.x2) / 2;
                    const yMid = (line.y1 + line.y2) / 2;
                    const angle = Math.atan2(line.y1 - line.y2, line.x1 - line.x2) * 180 / Math.PI;
                    return (
                        <div className='application-resource-tree__line' key={i}
                            style={{ width: distance, left: xMid - (distance / 2), top: yMid, transform: `translate(150px, 35px) rotate(${angle}deg)`}} />
                    );
                })}</div>
            ))}
        </div>
    );
};
