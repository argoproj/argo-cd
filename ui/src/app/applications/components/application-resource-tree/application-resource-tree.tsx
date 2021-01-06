import {DropDown, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as dagre from 'dagre';
import * as React from 'react';
import Moment from 'react-moment';

import * as models from '../../../shared/models';

import {EmptyState} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import {ApplicationURLs} from '../application-urls';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {ComparisonStatusIcon, getAppOverridesCount, HealthStatusIcon, isAppNode, NodeId, nodeKey} from '../utils';
import {NodeUpdateAnimation} from './node-update-animation';

function treeNodeKey(node: NodeId & {uid?: string}) {
    return node.uid || nodeKey(node);
}

const color = require('color');

require('./application-resource-tree.scss');

export interface ResourceTreeNode extends models.ResourceNode {
    status?: models.SyncStatusCode;
    health?: models.HealthStatus;
    hook?: boolean;
    root?: ResourceTreeNode;
    requiresPruning?: boolean;
    orphaned?: boolean;
}

export interface ApplicationResourceTreeProps {
    app: models.Application;
    tree: models.ApplicationTree;
    useNetworkingHierarchy: boolean;
    nodeFilter: (node: ResourceTreeNode) => boolean;
    selectedNodeFullName?: string;
    onNodeClick?: (fullName: string) => any;
    nodeMenu?: (node: models.ResourceNode) => React.ReactNode;
    onClearFilter: () => any;
    showOrphanedResources: boolean;
}

interface Line {
    x1: number;
    y1: number;
    x2: number;
    y2: number;
}

const NODE_WIDTH = 282;
const NODE_HEIGHT = 52;
const FILTERED_INDICATOR_NODE = '__filtered_indicator__';
const EXTERNAL_TRAFFIC_NODE = '__external_traffic__';
const INTERNAL_TRAFFIC_NODE = '__internal_traffic__';
const NODE_TYPES = {
    filteredIndicator: 'filtered_indicator',
    externalTraffic: 'external_traffic',
    externalLoadBalancer: 'external_load_balancer',
    internalTraffic: 'internal_traffic'
};

const BASE_COLORS = [
    '#0DADEA', // blue
    '#95D58F', // green
    '#F4C030', // orange
    '#FF6262', // red
    '#4B0082', // purple
    '#964B00' // brown
];

// generate lots of colors with different darkness
const TRAFFIC_COLORS = [0, 0.25, 0.4, 0.6]
    .map(darken =>
        BASE_COLORS.map(item =>
            color(item)
                .darken(darken)
                .hex()
        )
    )
    .reduce((first, second) => first.concat(second), []);

function getGraphSize(nodes: dagre.Node[]): {width: number; height: number} {
    let width = 0;
    let height = 0;
    nodes.forEach(node => {
        width = Math.max(node.x + node.width, width);
        height = Math.max(node.y + node.height, height);
    });
    return {width, height};
}

function filterGraph(app: models.Application, filteredIndicatorParent: string, graph: dagre.graphlib.Graph, predicate: (node: ResourceTreeNode) => boolean) {
    const appKey = appNodeKey(app);
    let filtered = 0;
    graph.nodes().forEach(nodeId => {
        const node: ResourceTreeNode = graph.node(nodeId) as any;
        const parentIds = graph.predecessors(nodeId);
        if (node.root != null && !predicate(node) && appKey !== nodeId) {
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
        graph.setNode(FILTERED_INDICATOR_NODE, {height: NODE_HEIGHT, width: NODE_WIDTH, count: filtered, type: NODE_TYPES.filteredIndicator});
        graph.setEdge(filteredIndicatorParent, FILTERED_INDICATOR_NODE);
    }
}

export function compareNodes(first: ResourceTreeNode, second: ResourceTreeNode) {
    function orphanedToInt(orphaned?: boolean) {
        return (orphaned && 1) || 0;
    }
    function compareRevision(a: string, b: string) {
        const numberA = Number(a);
        const numberB = Number(b);
        if (isNaN(numberA) || isNaN(numberB)) {
            return a.localeCompare(b);
        }
        return Math.sign(numberA - numberB);
    }
    function getRevision(a: ResourceTreeNode) {
        const filtered = a.info.filter(b => b.name === 'Revision' && b)[0];
        if (filtered == null) {
            return '';
        }
        const value = filtered.value;
        if (value == null) {
            return '';
        }
        return value.replace(/^Rev:/, '');
    }
    return (
        orphanedToInt(first.orphaned) - orphanedToInt(second.orphaned) ||
        nodeKey(first).localeCompare(nodeKey(second)) ||
        compareRevision(getRevision(first), getRevision(second)) ||
        0
    );
}

function appNodeKey(app: models.Application) {
    return nodeKey({group: 'argoproj.io', kind: app.kind, name: app.metadata.name, namespace: app.metadata.namespace});
}

function renderFilteredNode(node: {count: number} & dagre.Node, onClearFilter: () => any) {
    const indicators = new Array<number>();
    let count = Math.min(node.count - 1, 3);
    while (count > 0) {
        indicators.push(count--);
    }
    return (
        <React.Fragment>
            <div className='application-resource-tree__node' style={{left: node.x, top: node.y, width: node.width, height: node.height}}>
                <div className='application-resource-tree__node-kind-icon '>
                    <i className='icon fa fa-filter' />
                </div>
                <div className='application-resource-tree__node-content-wrap-overflow'>
                    <a className='application-resource-tree__node-title' onClick={onClearFilter}>
                        clear filters to show {node.count} additional resource{node.count > 1 && 's'}
                    </a>
                </div>
            </div>
            {indicators.map(i => (
                <div
                    key={i}
                    className='application-resource-tree__node application-resource-tree__filtered-indicator'
                    style={{left: node.x + i * 2, top: node.y + i * 2, width: node.width, height: node.height}}
                />
            ))}
        </React.Fragment>
    );
}

function renderTrafficNode(node: dagre.Node) {
    return (
        <div style={{position: 'absolute', left: 0, top: node.y, width: node.width, height: node.height}}>
            <div className='application-resource-tree__node-kind-icon' style={{fontSize: '2em'}}>
                <i className='icon fa fa-cloud' />
            </div>
        </div>
    );
}

function renderLoadBalancerNode(node: dagre.Node & {label: string; color: string}) {
    return (
        <div
            className='application-resource-tree__node application-resource-tree__node--load-balancer'
            style={{
                left: node.x,
                top: node.y,
                width: node.width,
                height: node.height
            }}>
            <div className='application-resource-tree__node-kind-icon'>
                <i title={node.kind} className={`icon fa fa-network-wired`} style={{color: node.color}} />
            </div>
            <div className='application-resource-tree__node-content'>
                <span className='application-resource-tree__node-title'>{node.label}</span>
            </div>
        </div>
    );
}

export const describeNode = (node: ResourceTreeNode) => {
    const lines = [`Kind: ${node.kind}`, `Namespace: ${node.namespace}`, `Name: ${node.name}`];
    if (node.images) {
        lines.push('Images:');
        node.images.forEach(i => lines.push(`- ${i}`));
    }
    return lines.join('\n');
};

function renderResourceNode(props: ApplicationResourceTreeProps, id: string, node: ResourceTreeNode & dagre.Node) {
    const fullName = nodeKey(node);
    let comparisonStatus: models.SyncStatusCode = null;
    let healthState: models.HealthStatus = null;
    if (node.status || node.health) {
        comparisonStatus = node.status;
        healthState = node.health;
    }
    const appNode = isAppNode(node);
    const rootNode = !node.root;
    return (
        <div
            onClick={() => props.onNodeClick && props.onNodeClick(fullName)}
            className={classNames('application-resource-tree__node', {
                'active': fullName === props.selectedNodeFullName,
                'application-resource-tree__node--orphaned': node.orphaned
            })}
            title={describeNode(node)}
            style={{left: node.x, top: node.y, width: node.width, height: node.height}}>
            {!appNode && <NodeUpdateAnimation resourceVersion={node.resourceVersion} />}
            <div
                className={classNames('application-resource-tree__node-kind-icon', {
                    'application-resource-tree__node-kind-icon--big': rootNode
                })}>
                <ResourceIcon kind={node.kind} />
                <br />
                {!rootNode && <div className='application-resource-tree__node-kind'>{ResourceLabel({kind: node.kind})}</div>}
            </div>
            <div className='application-resource-tree__node-content'>
                <span className='application-resource-tree__node-title'>{node.name}</span>
                <br />
                <span
                    className={classNames('application-resource-tree__node-status-icon', {
                        'application-resource-tree__node-status-icon--offset': rootNode
                    })}>
                    {node.hook && <i title='Resource lifecycle hook' className='fa fa-anchor' />}
                    {healthState != null && <HealthStatusIcon state={healthState} />}
                    {comparisonStatus != null && <ComparisonStatusIcon status={comparisonStatus} resource={!rootNode && node} />}
                    {appNode && !rootNode && (
                        <Consumer>
                            {ctx => (
                                <a href={ctx.baseHref + 'applications/' + node.name} title='Open application'>
                                    <i className='fa fa-external-link-alt' />
                                </a>
                            )}
                        </Consumer>
                    )}
                    <ApplicationURLs urls={rootNode ? props.app.status.summary.externalURLs : node.networkingInfo && node.networkingInfo.externalURLs} />
                </span>
            </div>
            <div className='application-resource-tree__node-labels'>
                {node.createdAt || rootNode ? (
                    <Moment className='application-resource-tree__node-label' fromNow={true} ago={true}>
                        {node.createdAt || props.app.metadata.creationTimestamp}
                    </Moment>
                ) : null}
                {(node.info || [])
                    .filter(tag => !tag.name.includes('Resource.') && !tag.name.includes('Node'))
                    .slice(0, 4)
                    .map((tag, i) => (
                        <span className='application-resource-tree__node-label' title={`${tag.name}:${tag.value}`} key={i}>
                            {tag.value}
                        </span>
                    ))}
                {(node.info || []).length > 4 && (
                    <Tooltip
                        content={(node.info || []).map(i => (
                            <div key={i.name}>
                                {i.name}: {i.value}
                            </div>
                        ))}
                        key={node.uid}>
                        <span className='application-resource-tree__node-label' title='More'>
                            More
                        </span>
                    </Tooltip>
                )}
            </div>
            {props.nodeMenu && (
                <div className='application-resource-tree__node-menu'>
                    <DropDown
                        isMenu={true}
                        anchor={() => (
                            <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                <i className='fa fa-ellipsis-v' />
                            </button>
                        )}>
                        {() => props.nodeMenu(node)}
                    </DropDown>
                </div>
            )}
        </div>
    );
}

function findNetworkTargets(nodes: ResourceTreeNode[], networkingInfo: models.ResourceNetworkingInfo): ResourceTreeNode[] {
    let result = new Array<ResourceTreeNode>();
    const refs = new Set((networkingInfo.targetRefs || []).map(nodeKey));
    result = result.concat(nodes.filter(target => refs.has(nodeKey(target))));
    if (networkingInfo.targetLabels) {
        result = result.concat(
            nodes.filter(target => {
                if (target.networkingInfo && target.networkingInfo.labels) {
                    return Object.keys(networkingInfo.targetLabels).every(key => networkingInfo.targetLabels[key] === target.networkingInfo.labels[key]);
                }
                return false;
            })
        );
    }
    return result;
}
export const ApplicationResourceTree = (props: ApplicationResourceTreeProps) => {
    const graph = new dagre.graphlib.Graph();
    graph.setGraph({nodesep: 15, rankdir: 'LR', marginx: -100});
    graph.setDefaultEdgeLabel(() => ({}));
    const overridesCount = getAppOverridesCount(props.app);
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
        info:
            overridesCount > 0
                ? [
                      {
                          name: 'Parameter overrides',
                          value: `${overridesCount} parameter override(s)`
                      }
                  ]
                : []
    };

    const statusByKey = new Map<string, models.ResourceStatus>();
    props.app.status.resources.forEach(res => statusByKey.set(nodeKey(res), res));
    const nodeByKey = new Map<string, ResourceTreeNode>();
    props.tree.nodes
        .map(node => ({...node, orphaned: false}))
        .concat(((props.showOrphanedResources && props.tree.orphanedNodes) || []).map(node => ({...node, orphaned: true})))
        .forEach(node => {
            const status = statusByKey.get(nodeKey(node));
            const resourceNode: ResourceTreeNode = {...node};
            if (status) {
                resourceNode.health = status.health;
                resourceNode.status = status.status;
                resourceNode.hook = status.hook;
                resourceNode.requiresPruning = status.requiresPruning;
            }
            nodeByKey.set(treeNodeKey(node), resourceNode);
        });
    const nodes = Array.from(nodeByKey.values());
    let roots: ResourceTreeNode[] = [];
    const childrenByParentKey = new Map<string, ResourceTreeNode[]>();
    if (props.useNetworkingHierarchy) {
        // Network view
        const hasParents = new Set<string>();
        const networkNodes = nodes.filter(node => node.networkingInfo);
        networkNodes.forEach(parent => {
            findNetworkTargets(networkNodes, parent.networkingInfo).forEach(child => {
                const children = childrenByParentKey.get(treeNodeKey(parent)) || [];
                hasParents.add(treeNodeKey(child));
                children.push(child);
                childrenByParentKey.set(treeNodeKey(parent), children);
            });
        });
        roots = networkNodes.filter(node => !hasParents.has(treeNodeKey(node)));
        const externalRoots = roots.filter(root => (root.networkingInfo.ingress || []).length > 0).sort(compareNodes);
        const internalRoots = roots.filter(root => (root.networkingInfo.ingress || []).length === 0).sort(compareNodes);
        const colorsBySource = new Map<string, string>();
        // sources are root internal services and external ingress/service IPs
        const sources = Array.from(
            new Set(
                internalRoots
                    .map(root => treeNodeKey(root))
                    .concat(
                        externalRoots.map(root => root.networkingInfo.ingress.map(ingress => ingress.hostname || ingress.ip)).reduce((first, second) => first.concat(second), [])
                    )
            )
        );
        // assign unique color to each traffic source
        sources.forEach((key, i) => colorsBySource.set(key, TRAFFIC_COLORS[i % TRAFFIC_COLORS.length]));

        if (externalRoots.length > 0) {
            graph.setNode(EXTERNAL_TRAFFIC_NODE, {height: NODE_HEIGHT, width: 30, type: NODE_TYPES.externalTraffic});
            externalRoots.sort(compareNodes).forEach(root => {
                const loadBalancers = root.networkingInfo.ingress.map(ingress => ingress.hostname || ingress.ip);
                processNode(root, root, loadBalancers.map(lb => colorsBySource.get(lb)));
                loadBalancers.forEach(key => {
                    const loadBalancerNodeKey = `${EXTERNAL_TRAFFIC_NODE}:${key}`;
                    graph.setNode(loadBalancerNodeKey, {
                        height: NODE_HEIGHT,
                        width: NODE_WIDTH,
                        type: NODE_TYPES.externalLoadBalancer,
                        label: key,
                        color: colorsBySource.get(key)
                    });
                    graph.setEdge(loadBalancerNodeKey, treeNodeKey(root), {colors: [colorsBySource.get(key)]});
                    graph.setEdge(EXTERNAL_TRAFFIC_NODE, loadBalancerNodeKey, {colors: [colorsBySource.get(key)]});
                });
            });
        }

        if (internalRoots.length > 0) {
            graph.setNode(INTERNAL_TRAFFIC_NODE, {height: NODE_HEIGHT, width: 30, type: NODE_TYPES.internalTraffic});
            internalRoots.forEach(root => {
                processNode(root, root, [colorsBySource.get(treeNodeKey(root))]);
                graph.setEdge(INTERNAL_TRAFFIC_NODE, treeNodeKey(root));
            });
        }
        if (props.nodeFilter) {
            // show filtered indicator next to external traffic node is app has it otherwise next to internal traffic node
            filterGraph(props.app, externalRoots.length > 0 ? EXTERNAL_TRAFFIC_NODE : INTERNAL_TRAFFIC_NODE, graph, props.nodeFilter);
        }
    } else {
        // Tree view
        const managedKeys = new Set(props.app.status.resources.map(nodeKey));
        const orphans: ResourceTreeNode[] = [];
        nodes.forEach(node => {
            if ((node.parentRefs || []).length === 0 || managedKeys.has(nodeKey(node))) {
                roots.push(node);
            } else {
                orphans.push(node);
                node.parentRefs.forEach(parent => {
                    const children = childrenByParentKey.get(treeNodeKey(parent)) || [];
                    children.push(node);
                    childrenByParentKey.set(treeNodeKey(parent), children);
                });
            }
        });
        roots.sort(compareNodes).forEach(node => {
            processNode(node, node);
            graph.setEdge(appNodeKey(props.app), treeNodeKey(node));
        });
        orphans.sort(compareNodes).forEach(node => {
            processNode(node, node);
        });
        graph.setNode(appNodeKey(props.app), {...appNode, width: NODE_WIDTH, height: NODE_HEIGHT});
        if (props.nodeFilter) {
            filterGraph(props.app, appNodeKey(props.app), graph, props.nodeFilter);
        }
    }

    function processNode(node: ResourceTreeNode, root: ResourceTreeNode, colors?: string[]) {
        graph.setNode(treeNodeKey(node), {...node, width: NODE_WIDTH, height: NODE_HEIGHT, root});
        (childrenByParentKey.get(treeNodeKey(node)) || []).sort(compareNodes).forEach(child => {
            if (treeNodeKey(child) === treeNodeKey(root)) {
                return;
            }
            graph.setEdge(treeNodeKey(node), treeNodeKey(child), {colors});
            processNode(child, root, colors);
        });
    }
    dagre.layout(graph);

    const edges: {from: string; to: string; lines: Line[]; backgroundImage?: string}[] = [];
    graph.edges().forEach(edgeInfo => {
        const edge = graph.edge(edgeInfo);
        const colors = (edge.colors as string[]) || [];
        let backgroundImage: string;
        if (colors.length > 0) {
            const step = 100 / colors.length;
            const gradient = colors.map((lineColor, i) => {
                return `${lineColor} ${step * i}%, ${lineColor} ${step * i + step / 2}%, transparent ${step * i + step / 2}%, transparent ${step * (i + 1)}%`;
            });
            backgroundImage = `linear-gradient(90deg, ${gradient})`;
        }

        const lines: Line[] = [];
        // don't render connections from hidden node representing internal traffic
        if (edgeInfo.v === INTERNAL_TRAFFIC_NODE || edgeInfo.w === INTERNAL_TRAFFIC_NODE) {
            return;
        }
        if (edge.points.length > 1) {
            for (let i = 1; i < edge.points.length; i++) {
                lines.push({x1: edge.points[i - 1].x, y1: edge.points[i - 1].y, x2: edge.points[i].x, y2: edge.points[i].y});
            }
        }
        edges.push({from: edgeInfo.v, to: edgeInfo.w, lines, backgroundImage});
    });
    const graphNodes = graph.nodes();
    const size = getGraphSize(graphNodes.map(id => graph.node(id)));
    return (
        (graphNodes.length === 0 && (
            <EmptyState icon=' fa fa-network-wired'>
                <h4>Your application has no network resources</h4>
                <h5>Try switching to tree or list view</h5>
            </EmptyState>
        )) || (
            <div
                className={classNames('application-resource-tree', {'application-resource-tree--network': props.useNetworkingHierarchy})}
                style={{width: size.width + 150, height: size.height + 250}}>
                {graphNodes.map(key => {
                    const node = graph.node(key);
                    const nodeType = node.type;
                    switch (nodeType) {
                        case NODE_TYPES.filteredIndicator:
                            return <React.Fragment key={key}>{renderFilteredNode(node as any, props.onClearFilter)}</React.Fragment>;
                        case NODE_TYPES.externalTraffic:
                            return <React.Fragment key={key}>{renderTrafficNode(node)}</React.Fragment>;
                        case NODE_TYPES.internalTraffic:
                            return null;
                        case NODE_TYPES.externalLoadBalancer:
                            return <React.Fragment key={key}>{renderLoadBalancerNode(node as any)}</React.Fragment>;
                        default:
                            return <React.Fragment key={key}>{renderResourceNode(props, key, node as ResourceTreeNode & dagre.Node)}</React.Fragment>;
                    }
                })}
                {edges.map(edge => (
                    <div key={`${edge.from}-${edge.to}`} className='application-resource-tree__edge'>
                        {edge.lines.map((line, i) => {
                            const distance = Math.sqrt(Math.pow(line.x1 - line.x2, 2) + Math.pow(line.y1 - line.y2, 2));
                            const xMid = (line.x1 + line.x2) / 2;
                            const yMid = (line.y1 + line.y2) / 2;
                            const angle = (Math.atan2(line.y1 - line.y2, line.x1 - line.x2) * 180) / Math.PI;
                            return (
                                <div
                                    className='application-resource-tree__line'
                                    key={i}
                                    style={{
                                        width: distance,
                                        left: xMid - distance / 2,
                                        top: yMid,
                                        backgroundImage: edge.backgroundImage,
                                        transform: `translate(150px, 35px) rotate(${angle}deg)`
                                    }}
                                />
                            );
                        })}
                    </div>
                ))}
            </div>
        )
    );
};
