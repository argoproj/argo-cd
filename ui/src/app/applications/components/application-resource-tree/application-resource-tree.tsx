import {DropDown, Tooltip} from 'argo-ui';
import classNames from 'classnames';
import * as dagre from 'dagre';
import * as React from 'react';
import Moment from 'react-moment';
import * as moment from 'moment';

import * as models from '../../../shared/models';
import {isValidManagedByURL, MANAGED_BY_URL_INVALID_TEXT, MANAGED_BY_URL_INVALID_COLOR} from '../../../shared/utils';

import {EmptyState} from '../../../shared/components';
import {AppContext, Consumer} from '../../../shared/context';
import {ApplicationURLs} from '../application-urls';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {
    BASE_COLORS,
    ComparisonStatusIcon,
    getAppOverridesCount,
    getApplicationSetOwnerRef,
    getAppSetHealthStatus,
    HealthStatusIcon,
    isApp,
    isAppNode,
    isAppSetNode,
    isYoungerThanXMinutes,
    NodeId,
    nodeKey,
    PodHealthIcon,
    getUsrMsgKeyToDisplay,
    getApplicationLinkURLFromNode,
    getManagedByURLFromNode,
    formatResourceInfo
} from '../utils';
import {NodeUpdateAnimation} from './node-update-animation';
import {PodGroup} from '../application-pod-view/pod-view';
import './application-resource-tree.scss';
import {ArrowConnector} from './arrow-connector';

function treeNodeKey(node: NodeId & {uid?: string}) {
    return node.uid || nodeKey(node);
}

export function nodeIdMatchesResourceKey(nodeId: string, targetKey: string, nodes: ResourceTreeNode[]): boolean {
    if (nodeId === targetKey) {
        return true;
    }
    const node = nodes.find(n => n.uid === nodeId || nodeKey(n) === nodeId);
    return !!node && nodeKey(node) === targetKey;
}

export function groupedNodeIdsContainKey(groupedNodeIds: string[], targetKey: string, nodes: ResourceTreeNode[]): boolean {
    if (!targetKey) {
        return false;
    }
    return groupedNodeIds.some(id => nodeIdMatchesResourceKey(id, targetKey, nodes));
}

const color = require('color');

export interface ResourceTreeNode extends models.ResourceNode {
    status?: models.SyncStatusCode;
    health?: models.HealthStatus;
    hook?: boolean;
    root?: ResourceTreeNode;
    requiresPruning?: boolean;
    orphaned?: boolean;
    podGroup?: PodGroup;
    isExpanded?: boolean;
}

export interface ApplicationResourceTreeProps {
    app: models.AbstractApplication;
    tree: models.ApplicationTree;
    useNetworkingHierarchy: boolean;
    nodeFilter: (node: ResourceTreeNode) => boolean;
    selectedNodeFullName?: string;
    onNodeClick?: (fullName: string) => any;
    onGroupdNodeClick?: (groupedNodeIds: string[]) => any;
    nodeMenu?: (node: models.ResourceNode) => React.ReactNode;
    onClearFilter: () => any;
    appContext?: AppContext;
    showOrphanedResources: boolean;
    showCompactNodes: boolean;
    userMsgs: models.UserMessages[];
    updateUsrHelpTipMsgs: (userMsgs: models.UserMessages) => void;
    setShowCompactNodes: (showCompactNodes: boolean) => void;
    zoom: number;
    podGroupCount: number;
    filters?: string[];
    setTreeFilterGraph?: (filterGraph: any[]) => void;
    nameDirection: boolean;
    nameWrap: boolean;
    setNodeExpansion: (node: string, isExpanded: boolean) => any;
    getNodeExpansion: (node: string) => boolean;
    showAppSetParent?: boolean;
}

interface Line {
    x1: number;
    y1: number;
    x2: number;
    y2: number;
}

// A large application hands dagre more nodes than it can lay out on the main thread: the cost is
// superlinear, so a few thousand resources freeze the tab for minutes. Draw a bounded set instead and
// report the remainder, rather than trying to draw everything faster.
const DEFAULT_VISIBLE_CAP = 200;
const MAX_CHILDREN_PER_PARENT = 25;

const NODE_WIDTH = 282;
const NODE_HEIGHT = 52;
const POD_NODE_HEIGHT = 136;
const POD_GROUP_ROW_HEIGHT = 20;
// Keep in sync with `$pods-per-row` in `application-resource-tree.scss`.
const POD_GROUP_PODS_PER_ROW = 8;
const FILTERED_INDICATOR_NODE = '__filtered_indicator__';
const CAPPED_INDICATOR_NODE = '__capped_indicator__';
// A synthetic parent standing for a whole kind. Giving the bulk kinds a parent of their own turns one
// contested budget into a budget per kind: the top level stops growing with the size of the
// application, and each kind can be explored on its own terms.
const KIND_GROUP_PREFIX = '__kind_group__/';
const KIND_GROUP_PREVIEW = 5;
// Every "+N more" grows by the same step, and the graph as a whole stops at a ceiling: expanding
// should cost a predictable amount rather than letting one click decide the size of the whole graph.
const EXPAND_STEP = 50;
const MAX_VISIBLE_CAP = 1000;
const KIND_CHIP_LIMIT = 6;
const EXTERNAL_TRAFFIC_NODE = '__external_traffic__';
const INTERNAL_TRAFFIC_NODE = '__internal_traffic__';
const NODE_TYPES = {
    filteredIndicator: 'filtered_indicator',
    cappedIndicator: 'capped_indicator',
    kindGroup: 'kind_group',
    externalTraffic: 'external_traffic',
    externalLoadBalancer: 'external_load_balancer',
    internalTraffic: 'internal_traffic',
    groupedNodes: 'grouped_nodes',
    podGroup: 'pod_group'
};

// generate lots of colors with different darkness
const TRAFFIC_COLORS = [0, 0.25, 0.4, 0.6].map(darken => BASE_COLORS.map(item => color(item).darken(darken).hex())).reduce((first, second) => first.concat(second), []);

function getGraphSize(nodes: dagre.Node[]): {width: number; height: number} {
    let width = 0;
    let height = 0;
    nodes.forEach(node => {
        width = Math.max(node.x + node.width, width);
        height = Math.max(node.y + node.height, height);
    });
    return {width, height};
}

// `totalByKind` carries how many resources of each kind the application has, for groups hanging off
// the application node. Without it a group reports how many of its kind were drawn, which on a
// bounded tree is a sample: "15 ConfigMaps" for an application that has four thousand.
function groupNodes(nodes: ResourceTreeNode[], graph: dagre.graphlib.Graph<{[key: string]: any}>, totalByKind?: Map<string, number>, appKey?: string) {
    function getNodeGroupingInfo(nodeId: string) {
        const node = graph.node(nodeId);
        return {
            nodeId,
            kind: node.kind,
            parentIds: graph.predecessors(nodeId),
            childIds: graph.successors(nodeId)
        };
    }

    function filterNoChildNode(nodeInfo: {childIds: dagre.Node[]}) {
        return nodeInfo.childIds.length === 0;
    }

    // create nodes array with parent/child nodeId
    const nodesInfoArr = graph.nodes().map(getNodeGroupingInfo);

    // group sibling nodes into a 2d array
    const siblingNodesArr = nodesInfoArr
        .reduce((acc, curr) => {
            if (curr.childIds.length > 1) {
                acc.push(curr.childIds.map(nodeId => getNodeGroupingInfo(nodeId.toString())));
            }
            return acc;
        }, [])
        .map(nodeArr => nodeArr.filter(filterNoChildNode));

    // group sibling nodes with same kind
    const groupedNodesArr = siblingNodesArr
        .map(eachLevel => {
            return eachLevel.reduce(
                (groupedNodesInfo: {kind: string; nodeIds?: string[]; parentIds?: dagre.Node[]}[], currentNodeInfo: {kind: string; nodeId: string; parentIds: dagre.Node[]}) => {
                    const index = groupedNodesInfo.findIndex((nodeInfo: {kind: string}) => currentNodeInfo.kind === nodeInfo.kind);
                    if (index > -1) {
                        groupedNodesInfo[index].nodeIds.push(currentNodeInfo.nodeId);
                    }

                    if (groupedNodesInfo.length === 0 || index < 0) {
                        const nodeIdArr = [];
                        nodeIdArr.push(currentNodeInfo.nodeId);
                        const groupedNodesInfoObj = {
                            kind: currentNodeInfo.kind,
                            nodeIds: nodeIdArr,
                            parentIds: currentNodeInfo.parentIds
                        };
                        groupedNodesInfo.push(groupedNodesInfoObj);
                    }

                    return groupedNodesInfo;
                },
                []
            );
        })
        .reduce((flattedNodesGroup, groupedNodes) => {
            return flattedNodesGroup.concat(groupedNodes);
        }, [])
        .filter((eachArr: {nodeIds: string[]}) => eachArr.nodeIds.length > 1);

    // update graph
    if (groupedNodesArr.length > 0) {
        groupedNodesArr.forEach((obj: {kind: string; nodeIds: string[]; parentIds: dagre.Node[]}) => {
            const {nodeIds, kind, parentIds} = obj;
            // Members previewed under a kind node are already a deliberate sample of that kind.
            if (parentIds[0].toString().startsWith(KIND_GROUP_PREFIX)) {
                return;
            }
            const groupedNodeIds: string[] = [];
            const podGroupIds: string[] = [];
            nodeIds.forEach((nodeId: string) => {
                const index = nodes.findIndex(node => nodeId === node.uid || nodeId === nodeKey(node));
                const graphNode = graph.node(nodeId);
                if (!graphNode?.podGroup && index > -1) {
                    groupedNodeIds.push(nodeId);
                } else {
                    podGroupIds.push(nodeId);
                }
            });
            const reducedNodeIds = nodeIds.reduce((acc, aNodeId) => {
                if (podGroupIds.findIndex(i => i === aNodeId) < 0) {
                    acc.push(aNodeId);
                }
                return acc;
            }, []);
            if (groupedNodeIds.length > 1) {
                groupedNodeIds.forEach(n => graph.removeNode(n));
                // Only groups directly under the application node stand for a whole kind; deeper ones
                // (ReplicaSets under a Deployment) mean exactly what they collapsed.
                const kindTotal = appKey && parentIds[0].toString() === appKey ? totalByKind?.get(kind) : undefined;
                graph.setNode(`${parentIds[0].toString()}/child/${kind}`, {
                    kind,
                    groupedNodeIds,
                    height: NODE_HEIGHT,
                    width: NODE_WIDTH,
                    count: reducedNodeIds.length,
                    kindTotal,
                    type: NODE_TYPES.groupedNodes
                });
                graph.setEdge(parentIds[0].toString(), `${parentIds[0].toString()}/child/${kind}`);
            }
        });
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
            return a.localeCompare(b, undefined, {numeric: true});
        }
        return Math.sign(numberA - numberB);
    }
    function getRevision(a: ResourceTreeNode) {
        const filtered = (a.info || []).filter(b => b.name === 'Revision' && b)[0];
        if (filtered == null) {
            return '';
        }
        const value = filtered.value;
        if (value == null) {
            return '';
        }
        return value.replace(/^Rev:/, '');
    }
    if (first.kind === 'ReplicaSet') {
        return (
            orphanedToInt(first.orphaned) - orphanedToInt(second.orphaned) ||
            compareRevision(getRevision(second), getRevision(first)) ||
            nodeKey(first).localeCompare(nodeKey(second), undefined, {numeric: true}) ||
            0
        );
    }
    return (
        orphanedToInt(first.orphaned) - orphanedToInt(second.orphaned) ||
        nodeKey(first).localeCompare(nodeKey(second), undefined, {numeric: true}) ||
        compareRevision(getRevision(first), getRevision(second)) ||
        0
    );
}

// How much attention a resource needs, lower first. Anything unhealthy outranks a healthy resource,
// and among healthy ones the workloads outrank configuration: otherwise a budget spent in name order
// fills up with ConfigMaps and never reaches the Deployment the user came to look at.
const RELEVANCE_WORKLOAD = 4;
const RELEVANCE_EXPOSURE = 5;
const RELEVANCE_UNREMARKABLE = 6;
const WORKLOAD_KINDS = new Set(['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob', 'Rollout', 'ReplicationController']);
const EXPOSURE_KINDS = new Set(['Service', 'Ingress', 'Gateway', 'HTTPRoute', 'GRPCRoute', 'Route']);

// State of a resource in one word, for summarising a group that is not drawn. Sync state stands in
// where a resource has no health of its own, which is most of a large application.
function summaryState(node: ResourceTreeNode): {state: string; health: boolean} {
    const health = node.health?.status;
    if (health && health !== models.HealthStatuses.Healthy) {
        return {state: health, health: true};
    }
    if (node.status === models.SyncStatuses.OutOfSync) {
        return {state: models.SyncStatuses.OutOfSync, health: false};
    }
    return health ? {state: health, health: true} : {state: models.SyncStatuses.Synced, health: false};
}

// Tally of what a marker hides, most concerning first, so the count can say whether opening it is
// worth it rather than only how much is behind it.
function tallyStates(nodes: ResourceTreeNode[]): {state: string; count: number; health: boolean}[] {
    const counts = new Map<string, {count: number; health: boolean}>();
    nodes.forEach(node => {
        const {state, health} = summaryState(node);
        const seen = counts.get(state);
        counts.set(state, {count: (seen?.count || 0) + 1, health});
    });
    const settled = (state: string) => state === models.SyncStatuses.Synced || state === models.HealthStatuses.Healthy;
    return Array.from(counts, ([state, tally]) => ({state, count: tally.count, health: tally.health})).sort(
        (a, b) => Number(settled(a.state)) - Number(settled(b.state)) || b.count - a.count
    );
}

function ownRelevance(node: ResourceTreeNode): number {
    switch (node.health?.status) {
        case models.HealthStatuses.Degraded:
        case models.HealthStatuses.Missing:
            return 0;
        case models.HealthStatuses.Progressing:
            return 1;
    }
    if (node.status === models.SyncStatuses.OutOfSync) {
        return 2;
    }
    switch (node.health?.status) {
        case models.HealthStatuses.Suspended:
        case models.HealthStatuses.Unknown:
            return 3;
    }
    if (WORKLOAD_KINDS.has(node.kind)) {
        return RELEVANCE_WORKLOAD;
    }
    if (EXPOSURE_KINDS.has(node.kind)) {
        return RELEVANCE_EXPOSURE;
    }
    return RELEVANCE_UNREMARKABLE;
}

// A parent is as interesting as the most interesting thing beneath it: a healthy looking Deployment
// whose pod is failing still needs to be on screen. Memoised per node, so the whole forest costs one
// traversal however many roots ask.
export function subtreeRelevance(
    node: ResourceTreeNode,
    childrenByParentKey: Map<string, ResourceTreeNode[]>,
    memo: Map<string, number> = new Map(),
    visiting: Set<string> = new Set()
): number {
    const key = treeNodeKey(node);
    const cached = memo.get(key);
    if (cached !== undefined) {
        return cached;
    }
    // Guards against a cycle in parentRefs, which would otherwise recurse forever.
    if (visiting.has(key)) {
        return RELEVANCE_UNREMARKABLE;
    }
    visiting.add(key);
    let best = ownRelevance(node);
    for (const child of childrenByParentKey.get(key) || []) {
        if (best === 0) {
            break;
        }
        best = Math.min(best, subtreeRelevance(child, childrenByParentKey, memo, visiting));
    }
    visiting.delete(key);
    memo.set(key, best);
    return best;
}

function appNodeKey(app: models.AbstractApplication) {
    return nodeKey({group: 'argoproj.io', kind: app.kind, name: app.metadata.name, namespace: app.metadata.namespace});
}

function renderKindGroupNode(node: {kind: string; total: number; group?: string} & dagre.Node, onDrillIntoKind: (kind: string) => any) {
    const plural = node.kind.endsWith('s') ? node.kind : `${node.kind}s`;
    return (
        <div
            className='application-resource-tree__node'
            title={`${node.total} ${plural} in this application — click to show only ${plural}`}
            onClick={() => onDrillIntoKind(node.kind)}
            style={{left: node.x, top: node.y, width: node.width, height: node.height, cursor: 'pointer'}}>
            <div className='application-resource-tree__node-kind-icon'>
                <ResourceIcon group={node.group || ''} kind={node.kind} />
                <br />
                <div className='application-resource-tree__node-kind'>{ResourceLabel({kind: node.kind})}</div>
            </div>
            <div className='application-resource-tree__node-content'>
                <div className='application-resource-tree__node-title'>{plural}</div>
                <div className='application-resource-tree__node-status-icon'>{node.total} total</div>
            </div>
        </div>
    );
}

function renderCappedNode(
    node: {
        shownCount: number;
        totalCount: number;
        hiddenCount: number;
        atCeiling?: boolean;
        hiddenStates?: {state: string; count: number; health: boolean}[];
        byKind?: {kind: string; count: number; shown: number}[];
        bucket?: string | null;
        parentKey?: string;
    } & dagre.Node,
    handlers: {
        onLoadMore: () => any;
        onSelectKind: (kind: string) => any;
        onClearBucket: () => any;
        onExpandParent: (parentKey: string, shownNow: number) => any;
        onShowAllKindChips: () => any;
    },
    showAllKinds: boolean
) {
    const states = (node.hiddenStates || []).slice(0, 3);
    // A per-parent overflow marker. Keyed on parentKey rather than on an empty kind breakdown, because
    // the application level card also has nothing to break down when the filters match nothing.
    if (node.parentKey) {
        return (
            <div
                className='application-resource-tree__node'
                title={
                    node.atCeiling
                        ? `${node.shownCount} of ${node.totalCount} shown — the graph is full, narrow with search or filter`
                        : `${node.shownCount} of ${node.totalCount} shown — click to load ${Math.min(EXPAND_STEP, node.hiddenCount)} more`
                }
                onClick={() => !node.atCeiling && handlers.onExpandParent(node.parentKey, node.shownCount)}
                style={{
                    left: node.x,
                    top: node.y,
                    width: node.width,
                    height: node.height,
                    borderStyle: 'dashed',
                    cursor: node.atCeiling ? 'default' : 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 12
                }}>
                <span style={{textAlign: 'center', lineHeight: '16px'}}>
                    {node.atCeiling ? (
                        <React.Fragment>
                            <span style={{opacity: 0.7}}>
                                {node.shownCount} of {node.totalCount} shown
                            </span>
                            <div style={{opacity: 0.7, fontSize: 11}}>graph is full — narrow with search or filter</div>
                        </React.Fragment>
                    ) : (
                        <React.Fragment>
                            <i className='fa fa-plus' style={{marginRight: 6}} />
                            {Math.min(EXPAND_STEP, node.hiddenCount)} more
                            <div style={{opacity: 0.7, fontSize: 11}}>
                                {node.shownCount} of {node.totalCount} shown
                            </div>
                            {states.length > 0 && (
                                <div style={{fontSize: 11, marginTop: 2, display: 'flex', gap: 8, justifyContent: 'center', alignItems: 'center'}}>
                                    {states.map(st => (
                                        <span key={st.state} title={`${st.count} ${st.state}`} style={{display: 'inline-flex', alignItems: 'center', gap: 3}}>
                                            {st.health ? (
                                                <HealthStatusIcon state={{status: st.state as models.HealthStatusCode, message: ''}} noSpin={true} />
                                            ) : (
                                                <ComparisonStatusIcon status={st.state as models.SyncStatusCode} noSpin={true} />
                                            )}
                                            {st.count}
                                        </span>
                                    ))}
                                </div>
                            )}
                        </React.Fragment>
                    )}
                </span>
            </div>
        );
    }
    const byKind = node.byKind || [];
    return (
        <div
            className='application-resource-tree__node'
            style={{left: node.x, top: node.y, width: node.width, height: node.height, borderStyle: 'dashed', padding: 8, overflow: 'hidden', fontSize: 12}}>
            <div style={{fontWeight: 600, marginBottom: 4}}>
                <i className='fa fa-layer-group' style={{marginRight: 6}} />
                {node.totalCount === 0 ? 'No resources match the current filters' : `Showing ${node.shownCount} of ${node.totalCount} resources`}
                {node.bucket ? ` · ${node.bucket}` : ''}
            </div>
            <div style={{opacity: 0.7, marginBottom: 6}}>
                {node.totalCount === 0 ? 'Clear a filter, or pick a different kind, to see resources again.' : 'Use search or filter to find a specific one.'}
            </div>
            <div style={{marginBottom: 6, lineHeight: '20px', maxHeight: showAllKinds ? 54 : undefined, overflowY: showAllKinds ? 'auto' : undefined}}>
                {(showAllKinds ? byKind : byKind.slice(0, KIND_CHIP_LIMIT)).map(k => (
                    <a
                        key={k.kind}
                        // Both numbers, because a count of what exists reads as a count of what is drawn.
                        title={`${k.shown} of ${k.count} ${k.kind} shown — click to show only ${k.kind}`}
                        onClick={() => handlers.onSelectKind(k.kind)}
                        style={{
                            display: 'inline-block',
                            margin: '0 4px 2px 0',
                            padding: '0 6px',
                            border: '1px solid currentColor',
                            borderRadius: 10,
                            opacity: 0.8,
                            cursor: 'pointer',
                            whiteSpace: 'nowrap'
                        }}>
                        {k.kind} ({k.shown}/{k.count})
                    </a>
                ))}
                {!showAllKinds && byKind.length > KIND_CHIP_LIMIT && (
                    <a
                        title='Show the remaining kinds'
                        onClick={handlers.onShowAllKindChips}
                        style={{display: 'inline-block', margin: '0 4px 2px 0', padding: '0 6px', cursor: 'pointer', whiteSpace: 'nowrap'}}>
                        +{byKind.length - KIND_CHIP_LIMIT} more kinds
                    </a>
                )}
            </div>
            <div>
                {node.hiddenCount > 0 && !node.atCeiling && (
                    <a onClick={handlers.onLoadMore} style={{marginRight: 12, cursor: 'pointer'}}>
                        <i className='fa fa-plus' style={{marginRight: 4}} />
                        Load {EXPAND_STEP} more
                    </a>
                )}
                {node.hiddenCount > 0 && node.atCeiling && (
                    <span style={{marginRight: 12, opacity: 0.7}}>Showing the most the graph can display — narrow with search or filter.</span>
                )}
                {node.bucket && (
                    <a onClick={handlers.onClearBucket} style={{cursor: 'pointer'}}>
                        <i className='fa fa-times' style={{marginRight: 4}} />
                        Show all kinds
                    </a>
                )}
            </div>
        </div>
    );
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

function renderGroupedNodes(
    props: ApplicationResourceTreeProps,
    node: {count: number; kindTotal?: number; groupedNodeIds: string[]} & dagre.Node & ResourceTreeNode,
    allNodes: ResourceTreeNode[]
) {
    const indicators = new Array<number>();
    let count = Math.min(node.count - 1, 3);
    while (count > 0) {
        indicators.push(count--);
    }
    const isActive = groupedNodeIdsContainKey(node.groupedNodeIds, props.selectedNodeFullName || '', allNodes);
    return (
        <React.Fragment>
            <div className={classNames('application-resource-tree__node', {active: isActive})} style={{left: node.x, top: node.y, width: node.width, height: node.height}}>
                <div className='application-resource-tree__node-kind-icon'>
                    <ResourceIcon group={node.group} kind={node.kind} />
                    <br />
                    <div className='application-resource-tree__node-kind'>{ResourceLabel({kind: node.kind})}</div>
                </div>
                <div
                    className='application-resource-tree__node-title application-resource-tree__direction-center-left'
                    onClick={() => props.onGroupdNodeClick && props.onGroupdNodeClick(node.groupedNodeIds)}
                    title={`Click to see details of ${node.count} collapsed ${node.kind} and doesn't contains any active pods`}>
                    {node.kindTotal ?? node.count} {node.kind.endsWith('s') ? node.kind : `${node.kind}s`}
                    <span style={{paddingLeft: '.5em', fontSize: 'small'}}>
                        {node.kind === 'ReplicaSet' ? (
                            <i
                                className='fa-solid fa-cart-flatbed icon-background'
                                title={`Click to see details of ${node.count} collapsed ${node.kind} and doesn't contains any active pods`}
                                key={node.uid}
                            />
                        ) : (
                            <i className='fa fa-info-circle icon-background' title={`Click to see details of ${node.count} collapsed ${node.kind}`} key={node.uid} />
                        )}
                    </span>
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

function renderLoadBalancerNode(node: dagre.Node & {label: string; color: string; kind: string}) {
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
    const lines = [`Kind: ${node.kind}`, `Namespace: ${node.namespace || '(global)'}`, `Name: ${node.name}`];
    if (node.images) {
        lines.push('Images:');
        node.images.forEach(i => lines.push(`- ${i}`));
    }
    return lines.join('\n');
};

function processPodGroup(targetPodGroup: ResourceTreeNode, child: ResourceTreeNode, props: ApplicationResourceTreeProps) {
    if (!targetPodGroup.podGroup) {
        const fullName = nodeKey(targetPodGroup);
        if ((targetPodGroup.parentRefs || []).length === 0) {
            targetPodGroup.root = targetPodGroup;
        }
        targetPodGroup.podGroup = {
            pods: [] as models.Pod[],
            fullName,
            ...targetPodGroup.podGroup,
            ...targetPodGroup,
            info: (targetPodGroup.info || []).filter(i => !i.name.includes('Resource.')),
            createdAt: targetPodGroup.createdAt,
            renderMenu: () => props.nodeMenu(targetPodGroup),
            kind: targetPodGroup.kind,
            type: 'parentResource',
            name: targetPodGroup.name
        };
    }
    if (child.kind === 'Pod') {
        const p: models.Pod = {
            ...child,
            fullName: nodeKey(child),
            metadata: {name: child.name},
            spec: {nodeName: 'Unknown'},
            health: child.health ? child.health.status : 'Unknown'
        } as models.Pod;

        // Get node name for Pod
        child.info?.forEach(i => {
            if (i.name === 'Node') {
                p.spec.nodeName = i.value;
            }
        });
        targetPodGroup.podGroup.pods.push(p);
    }
}

function getPodGroupNumberOfRows(pods: models.Pod[], showPodGroupByStatus: boolean) {
    if (!pods || pods.length === 0) {
        return 0;
    }
    if (showPodGroupByStatus) {
        const statuses = new Set<string>();
        for (const pod of pods) {
            if (!pod) {
                continue;
            }
            const health = pod.health;
            if (health === 'Healthy' || health === 'Degraded' || health === 'Progressing') {
                statuses.add(health);
            }
        }
        return statuses.size;
    }
    return Math.ceil(pods.length / POD_GROUP_PODS_PER_ROW);
}

function renderPodGroup(
    props: ApplicationResourceTreeProps,
    node: ResourceTreeNode & dagre.Node & {groupedNodeIds?: string[]},
    childMap: Map<string, ResourceTreeNode[]>,
    showPodGroupByStatus: boolean
) {
    const fullName = nodeKey(node);
    let comparisonStatus: models.SyncStatusCode = null;
    let healthState: models.HealthStatus = null;
    if (node.status || node.health) {
        comparisonStatus = node.status;
        healthState = node.health;
    }
    const appNode = isAppNode(node);
    const rootNode = !node.root;
    const extLinks: string[] = isApp(props.app) ? (props.app as models.Application).status.summary.externalURLs : [];
    const podGroupChildren = childMap.get(treeNodeKey(node));
    const nonPodChildren = podGroupChildren?.reduce((acc, child) => {
        if (child.kind !== 'Pod') {
            acc.push(child);
        }
        return acc;
    }, []);
    const childCount = nonPodChildren?.length;
    const podGroup = node.podGroup;
    const podGroupHealthy = [];
    const podGroupDegraded = [];
    const podGroupInProgress = [];

    for (const pod of podGroup?.pods || []) {
        switch (pod.health) {
            case 'Healthy':
                podGroupHealthy.push(pod);
                break;
            case 'Degraded':
                podGroupDegraded.push(pod);
                break;
            case 'Progressing':
                podGroupInProgress.push(pod);
        }
    }

    // Use Dagre's measured height directly to avoid duplicating sizing logic in the render path.
    // Dagre assigns node.y as the node center; convert to DOM top-left for rendering.
    const podGroupHeight = node.height;
    const podGroupTop = node.y - podGroupHeight / 2;

    return (
        <div
            className={classNames('application-resource-tree__node', {
                'active': fullName === props.selectedNodeFullName && !rootNode,
                'application-resource-tree__node--orphaned': node.orphaned,
                'application-resource-tree__node--grouped-node': !showPodGroupByStatus
            })}
            title={describeNode(node)}
            style={{
                left: node.x,
                top: podGroupTop,
                width: node.width,
                height: podGroupHeight
            }}>
            <NodeUpdateAnimation resourceVersion={node.resourceVersion} />
            <div onClick={() => props.onNodeClick && props.onNodeClick(fullName)} className={`application-resource-tree__node__top-part`}>
                <div
                    className={classNames('application-resource-tree__node-kind-icon', {
                        'application-resource-tree__node-kind-icon--big': rootNode
                    })}>
                    <ResourceIcon group={node.group} kind={node.kind || 'Unknown'} />
                    <br />
                    {!rootNode && <div className='application-resource-tree__node-kind'>{ResourceLabel({kind: node.kind})}</div>}
                </div>
                <div
                    className={classNames('application-resource-tree__node-content', {
                        'application-resource-tree__fullname': props.nameWrap,
                        'application-resource-tree__wrappedname': !props.nameWrap
                    })}>
                    <span
                        className={classNames('application-resource-tree__node-title', {
                            'application-resource-tree__direction-right': props.nameDirection,
                            'application-resource-tree__direction-left': !props.nameDirection
                        })}
                        onClick={() => props.onGroupdNodeClick && props.onGroupdNodeClick(node.groupedNodeIds)}>
                        {node.name}
                    </span>
                    <span
                        className={classNames('application-resource-tree__node-status-icon', {
                            'application-resource-tree__node-status-icon--offset': rootNode
                        })}>
                        {node.hook && <i title='Resource lifecycle hook' className='fa fa-anchor' />}
                        {healthState != null && <HealthStatusIcon state={healthState} />}
                        {comparisonStatus != null && <ComparisonStatusIcon status={comparisonStatus} resource={!rootNode && node} />}
                        {appNode && !rootNode && (
                            <Consumer>
                                {ctx => {
                                    // For nested applications, use the node's data to construct the URL
                                    const linkInfo = getApplicationLinkURLFromNode(node, ctx.baseHref);
                                    const managedByURL = getManagedByURLFromNode(node);
                                    const managedByURLInvalid = !!managedByURL && !isValidManagedByURL(managedByURL);
                                    if (managedByURLInvalid) {
                                        return (
                                            <span
                                                role='link'
                                                aria-disabled={true}
                                                style={{cursor: 'not-allowed', display: 'inline-flex', alignItems: 'center'}}
                                                onClick={e => e.stopPropagation()}
                                                title={`Open application\n${MANAGED_BY_URL_INVALID_TEXT}`}>
                                                <i className='fa fa-window-maximize' style={{color: MANAGED_BY_URL_INVALID_COLOR}} />
                                            </span>
                                        );
                                    }
                                    return (
                                        <a
                                            href={linkInfo.url}
                                            target={linkInfo.isExternal ? '_blank' : undefined}
                                            rel={linkInfo.isExternal ? 'noopener noreferrer' : undefined}
                                            onClick={e => {
                                                e.stopPropagation();
                                            }}
                                            title={managedByURL ? `Open application\nmanaged-by-url: ${managedByURL}` : 'Open application'}>
                                            <i className='fa fa-window-maximize' />
                                        </a>
                                    );
                                }}
                            </Consumer>
                        )}
                        {isAppSetNode(node) && (
                            <a onClick={e => e.stopPropagation()} title='Open ApplicationSet'>
                                <i className='fa fa-external-link-alt' />
                            </a>
                        )}
                        <ApplicationURLs urls={rootNode ? extLinks : node.networkingInfo && node.networkingInfo.externalURLs} />
                    </span>
                    {childCount > 0 && (
                        <>
                            <br />
                            <div
                                style={{top: podGroupHeight / 2 - 6}}
                                className='application-resource-tree__node--podgroup--expansion'
                                onClick={event => {
                                    expandCollapse(node, props);
                                    event.stopPropagation();
                                }}>
                                {props.getNodeExpansion(node.uid) ? <div className='fa fa-minus' /> : <div className='fa fa-plus' />}
                            </div>
                        </>
                    )}
                </div>
                <div className='application-resource-tree__node-labels'>
                    {node.createdAt || rootNode ? (
                        <Moment className='application-resource-tree__node-label' fromNow={true} ago={true}>
                            {node.createdAt || props.app.metadata.creationTimestamp}
                        </Moment>
                    ) : null}
                    {(node.info || [])
                        .filter(tag => !tag.name.includes('Node'))
                        .slice(0, 4)
                        .map((tag, i) => (
                            <span className='application-resource-tree__node-label' title={`${tag.name}:${tag.value}`} key={i}>
                                {tag.value}
                            </span>
                        ))}
                    {(node.info || []).length > 4 && (
                        <Tooltip
                            content={
                                <>
                                    {(node.info || []).map(i => {
                                        // Use common formatting function for CPU and Memory
                                        if (i.name === 'cpu' || i.name === 'memory') {
                                            const {tooltipValue} = formatResourceInfo(i.name, `${i.value}`);
                                            return <div key={i.name}>{tooltipValue}</div>;
                                        } else {
                                            return (
                                                <div key={i.name}>
                                                    {i.name}: {i.value}
                                                </div>
                                            );
                                        }
                                    })}
                                </>
                            }
                            key={node.uid}>
                            <span className='application-resource-tree__node-label' title='More'>
                                More
                            </span>
                        </Tooltip>
                    )}
                </div>
                {props.nodeMenu && !isAppSetNode(node) && (
                    <div className='application-resource-tree__node-menu'>
                        <DropDown
                            key={node.uid}
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
            <div className='application-resource-tree__node--lower-section'>
                {[podGroupHealthy, podGroupDegraded, podGroupInProgress].map((pods, index) => {
                    if (pods.length > 0) {
                        return (
                            <div key={index} className={`application-resource-tree__node--lower-section__pod-group`}>
                                {renderPodGroupByStatus(props, node, pods, showPodGroupByStatus)}
                            </div>
                        );
                    }
                })}
            </div>
        </div>
    );
}

function renderPodGroupByStatus(props: ApplicationResourceTreeProps, node: any, pods: models.Pod[], showPodGroupByStatus: boolean) {
    return (
        <div className='application-resource-tree__node--lower-section__pod-group__pod-container__pods'>
            {pods.length !== 0 && showPodGroupByStatus ? (
                <React.Fragment>
                    <div className={`pod-view__node__pod pod-view__node__pod--${pods[0].health.toLowerCase()}`}>
                        <PodHealthIcon state={{status: pods[0].health, message: ''}} key={pods[0].uid} />
                    </div>

                    <div className='pod-view__node__label--large'>
                        <a
                            className='application-resource-tree__node-title'
                            onClick={() =>
                                props.onGroupdNodeClick && props.onGroupdNodeClick(node.groupdedNodeIds === 'undefined' ? node.groupdedNodeIds : pods.map(pod => pod.uid))
                            }>
                            &nbsp;
                            <span title={`Click to view the ${pods[0].health.toLowerCase()} pods list`}>
                                {pods[0].health} {pods.length} pods
                            </span>
                        </a>
                    </div>
                </React.Fragment>
            ) : (
                pods.map(
                    pod =>
                        props.nodeMenu && (
                            <DropDown
                                key={pod.uid}
                                isMenu={true}
                                anchor={() => (
                                    <Tooltip
                                        content={
                                            <div>
                                                {pod.metadata.name}
                                                <div>Health: {pod.health}</div>
                                                {pod.createdAt && (
                                                    <span>
                                                        <span>Created: </span>
                                                        <Moment fromNow={true} ago={true}>
                                                            {pod.createdAt}
                                                        </Moment>
                                                        <span> ago ({<Moment local={true}>{pod.createdAt}</Moment>})</span>
                                                    </span>
                                                )}
                                            </div>
                                        }
                                        popperOptions={{
                                            modifiers: {
                                                preventOverflow: {
                                                    enabled: true
                                                },
                                                hide: {
                                                    enabled: false
                                                },
                                                flip: {
                                                    enabled: false
                                                }
                                            }
                                        }}
                                        key={pod.metadata.name}>
                                        <div style={{position: 'relative'}}>
                                            {isYoungerThanXMinutes(pod, 30) && (
                                                <i className='fas fa-star application-resource-tree__node--lower-section__pod-group__pod application-resource-tree__node--lower-section__pod-group__pod__star-icon' />
                                            )}
                                            <div
                                                className={`application-resource-tree__node--lower-section__pod-group__pod application-resource-tree__node--lower-section__pod-group__pod--${pod.health.toLowerCase()}`}>
                                                <PodHealthIcon state={{status: pod.health, message: ''}} />
                                            </div>
                                        </div>
                                    </Tooltip>
                                )}>
                                {() => props.nodeMenu(pod)}
                            </DropDown>
                        )
                )
            )}
        </div>
    );
}

function expandCollapse(node: ResourceTreeNode, props: ApplicationResourceTreeProps) {
    const isExpanded = !props.getNodeExpansion(node.uid);
    node.isExpanded = isExpanded;
    props.setNodeExpansion(node.uid, isExpanded);
}

function NodeInfoDetails({tag: tag, kind: kind}: {tag: models.InfoItem; kind: string}) {
    if (kind === 'Pod') {
        const val = tag.name;
        if (val === 'Status Reason') {
            if (String(tag.value) !== 'ImagePullBackOff')
                return (
                    <span className='application-resource-tree__node-label' title={`Status: ${tag.value}`}>
                        {tag.value}
                    </span>
                );
            else {
                return (
                    <span
                        className='application-resource-tree__node-label'
                        title='One of the containers may have the incorrect image name/tag, or you may be fetching from the incorrect repository, or the repository requires authentication.'>
                        {tag.value}
                    </span>
                );
            }
        } else if (val === 'Containers') {
            const arr = String(tag.value).split('/');
            const title = `Number of containers in total: ${arr[1]} \nNumber of ready containers: ${arr[0]}`;
            return (
                <span className='application-resource-tree__node-label' title={title}>
                    {tag.value}
                </span>
            );
        } else if (val === 'Restart Count') {
            return (
                <span className='application-resource-tree__node-label' title={`The total number of restarts of the containers: ${tag.value}`}>
                    {tag.value}
                </span>
            );
        } else if (val === 'Revision') {
            return (
                <span className='application-resource-tree__node-label' title={`The revision in which pod present is: ${tag.value}`}>
                    {tag.value}
                </span>
            );
        } else if (val === 'cpu' || val === 'memory') {
            // Use common formatting function for CPU and Memory
            const {displayValue, tooltipValue} = formatResourceInfo(val, String(tag.value));
            return (
                <span className='application-resource-tree__node-label' title={tooltipValue}>
                    {displayValue}
                </span>
            );
        } else {
            return (
                <span className='application-resource-tree__node-label' title={`${tag.name}: ${tag.value}`}>
                    {tag.value}
                </span>
            );
        }
    } else {
        return (
            <span className='application-resource-tree__node-label' title={`${tag.name}: ${tag.value}`}>
                {tag.value}
            </span>
        );
    }
}

function renderResourceNode(props: ApplicationResourceTreeProps, node: ResourceTreeNode & dagre.Node, nodesHavingChildren: Map<string, number>) {
    const fullName = nodeKey(node);
    let comparisonStatus: models.SyncStatusCode = null;
    let healthState: models.HealthStatus = null;
    if (node.status || node.health) {
        comparisonStatus = node.status;
        healthState = node.health;
    }
    const appNode = isAppNode(node);
    const rootNode = !node.root;
    const extLinks: string[] = isApp(props.app) ? (props.app as models.Application).status.summary.externalURLs : [];
    const childCount = nodesHavingChildren.get(node.uid);
    const ownerAppSetRef = rootNode && appNode && isApp(props.app) ? getApplicationSetOwnerRef(props.app as models.Application) : null;
    const isAppSetParent = isAppSetNode(node) && isApp(props.app) && getApplicationSetOwnerRef(props.app as models.Application)?.name === node.name;
    const isManagedAppSet = isAppSetNode(node) && !isAppSetParent;
    return (
        <div
            onClick={() => props.onNodeClick && props.onNodeClick(fullName)}
            className={classNames('application-resource-tree__node', !isManagedAppSet && 'application-resource-tree__node--' + node.kind.toLowerCase(), {
                'active': fullName === props.selectedNodeFullName && !rootNode,
                'application-resource-tree__node--orphaned': node.orphaned
            })}
            title={isAppSetParent ? `ApplicationSet: ${node.name}\nThis ApplicationSet generates and manages this Application.` : describeNode(node)}
            style={{
                left: node.x,
                top: node.y,
                width: node.width,
                height: node.height
            }}>
            {!appNode && <NodeUpdateAnimation resourceVersion={node.resourceVersion} />}
            <div
                className={classNames('application-resource-tree__node-kind-icon', {
                    'application-resource-tree__node-kind-icon--big': rootNode
                })}>
                <ResourceIcon group={node.group} kind={node.kind} />
                <br />
                {!rootNode && <div className='application-resource-tree__node-kind'>{ResourceLabel({kind: node.kind})}</div>}
            </div>
            <div
                className={classNames('application-resource-tree__node-content', {
                    'application-resource-tree__fullname': props.nameWrap,
                    'application-resource-tree__wrappedname': !props.nameWrap
                })}>
                <div
                    className={classNames('application-resource-tree__node-title', {
                        'application-resource-tree__direction-right': props.nameDirection,
                        'application-resource-tree__direction-left': !props.nameDirection
                    })}>
                    {node.name}
                </div>
                <div
                    className={classNames('application-resource-tree__node-status-icon', {
                        'application-resource-tree__node-status-icon--offset': rootNode
                    })}>
                    {node.hook && <i title='Resource lifecycle hook' className='fa fa-anchor' />}
                    {healthState != null && <HealthStatusIcon state={healthState} />}
                    {comparisonStatus != null && <ComparisonStatusIcon status={comparisonStatus} resource={!rootNode && node} />}
                    {appNode && !rootNode && (
                        <Consumer>
                            {ctx => {
                                // For nested applications, use the node's data to construct the URL
                                const linkInfo = getApplicationLinkURLFromNode(node, ctx.baseHref);
                                const managedByURL = getManagedByURLFromNode(node);
                                const managedByURLInvalid = !!managedByURL && !isValidManagedByURL(managedByURL);
                                if (managedByURLInvalid) {
                                    return (
                                        <span
                                            role='link'
                                            aria-disabled={true}
                                            style={{cursor: 'not-allowed', display: 'inline-flex', alignItems: 'center'}}
                                            onClick={e => e.stopPropagation()}
                                            title={`Open application\n${MANAGED_BY_URL_INVALID_TEXT}`}>
                                            <i className='fa fa-window-maximize' style={{color: MANAGED_BY_URL_INVALID_COLOR}} />
                                        </span>
                                    );
                                }
                                return (
                                    <a
                                        href={linkInfo.url}
                                        target={linkInfo.isExternal ? '_blank' : undefined}
                                        rel={linkInfo.isExternal ? 'noopener noreferrer' : undefined}
                                        onClick={e => {
                                            e.stopPropagation();
                                        }}
                                        title={managedByURL ? `Open application\nmanaged-by-url: ${managedByURL}` : 'Open application'}>
                                        <i className='fa fa-window-maximize' />
                                    </a>
                                );
                            }}
                        </Consumer>
                    )}

                    <ApplicationURLs urls={isAppSetParent ? [] : rootNode ? extLinks : node.networkingInfo && node.networkingInfo.externalURLs} />
                </div>
                {childCount > 0 && (
                    <div
                        className='application-resource-tree__node--expansion'
                        onClick={event => {
                            expandCollapse(node, props);
                            event.stopPropagation();
                        }}>
                        {props.getNodeExpansion(node.uid) ? <div className='fa fa-minus' /> : <div className='fa fa-plus' />}
                    </div>
                )}
            </div>
            <div className='application-resource-tree__node-labels'>
                {ownerAppSetRef && !props.showAppSetParent && (
                    <Consumer>
                        {ctx => (
                            <a
                                className='application-resource-tree__node-label application-resource-tree__node-label--appset'
                                onClick={e => {
                                    e.stopPropagation();
                                    ctx.navigation.goto(`/applicationsets/${props.app.metadata.namespace}/${ownerAppSetRef.name}`);
                                }}
                                title={`Managed by ApplicationSet: ${ownerAppSetRef.name}`}>
                                {ownerAppSetRef.name}
                            </a>
                        )}
                    </Consumer>
                )}
                {isManagedAppSet && (
                    <Consumer>
                        {ctx => (
                            <a
                                className='application-resource-tree__node-label application-resource-tree__node-label--appset'
                                onClick={e => {
                                    e.stopPropagation();
                                    ctx.navigation.goto(`/applicationsets/${node.namespace}/${node.name}`);
                                }}
                                title={`View ApplicationSet: ${node.name}`}>
                                {node.name}
                            </a>
                        )}
                    </Consumer>
                )}
                {node.createdAt || rootNode ? (
                    <span title={`${node.kind} was created ${moment(node.createdAt || props.app.metadata.creationTimestamp).fromNow()}`}>
                        <Moment className='application-resource-tree__node-label' fromNow={true} ago={true}>
                            {node.createdAt || props.app.metadata.creationTimestamp}
                        </Moment>
                    </span>
                ) : null}
                {(node.info || [])
                    .filter(tag => !tag.name.includes('Node') && tag.name !== 'managed-by-url')
                    .slice(0, 2)
                    .map((tag, i) => {
                        return <NodeInfoDetails tag={tag} kind={node.kind} key={i} />;
                    })}
                {(node.info || []).length > 3 && (
                    <Tooltip
                        content={
                            <>
                                {(node.info || []).map(i => {
                                    // Use common formatting function for CPU and Memory
                                    if (i.name === 'cpu' || i.name === 'memory') {
                                        const {tooltipValue} = formatResourceInfo(i.name, `${i.value}`);
                                        return <div key={i.name}>{tooltipValue}</div>;
                                    } else {
                                        return (
                                            <div key={i.name}>
                                                {i.name}: {i.value}
                                            </div>
                                        );
                                    }
                                })}
                            </>
                        }
                        key={node.uid}>
                        <span className='application-resource-tree__node-label' title='More'>
                            More
                        </span>
                    </Tooltip>
                )}
            </div>
            {props.nodeMenu && !isAppSetParent && (
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
    const graph = new dagre.graphlib.Graph<{[key: string]: any}>();
    // How much of the budget this render has spent, and how much it turned away.
    const renderState = {drawn: 0, skipped: 0};
    graph.setGraph({nodesep: 25, rankdir: 'LR', marginy: 45, marginx: -100, ranksep: 80});
    graph.setDefaultEdgeLabel(() => ({}));
    const overridesCount = getAppOverridesCount(props.app);
    const appNode = {
        kind: props.app.kind,
        name: props.app.metadata.name,
        namespace: props.app.metadata.namespace,
        resourceVersion: props.app.metadata.resourceVersion,
        group: 'argoproj.io',
        version: '',
        // @ts-expect-error its not any
        children: [],
        status: isApp(props.app) ? (props.app as models.Application).status.sync.status : null,
        health: isApp(props.app) ? (props.app as models.Application).status.health : {status: getAppSetHealthStatus(props.app as models.ApplicationSet), message: ''},
        uid: props.app.kind + '-' + props.app.metadata.namespace + '-' + props.app.metadata.name,
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

    const appSetRef = isApp(props.app) && props.showAppSetParent ? getApplicationSetOwnerRef(props.app as models.Application) : null;
    const appSetNode = appSetRef
        ? {
              kind: 'ApplicationSet',
              name: appSetRef.name,
              namespace: props.app.metadata.namespace,
              group: 'argoproj.io',
              version: '',
              children: [] as string[],
              status: null as string,
              health: null as models.HealthStatus,
              uid: 'ApplicationSet-' + props.app.metadata.namespace + '-' + appSetRef.name,
              info: [] as {name: string; value: string}[]
          }
        : null;

    const statusByKey = new Map<string, models.ResourceStatus>();
    const appSetStatusByKey = new Map<string, models.ApplicationSetResource>();
    if (isApp(props.app)) {
        (props.app as models.Application).status.resources.forEach(res => statusByKey.set(nodeKey(res), res));
    } else if ((props.app as models.ApplicationSet).status?.resources) {
        (props.app as models.ApplicationSet).status.resources.forEach(res => appSetStatusByKey.set(nodeKey(res), res));
    }
    const nodeByKey = new Map<string, ResourceTreeNode>();
    props.tree.nodes
        .map(node => ({...node, orphaned: false}))
        .concat(((props.showOrphanedResources && props.tree.orphanedNodes) || []).map(node => ({...node, orphaned: true})))
        .forEach(node => {
            const resourceNode: ResourceTreeNode = {...node};
            if (isApp(props.app)) {
                const status = statusByKey.get(nodeKey(node));
                if (status) {
                    resourceNode.health = status.health;
                    resourceNode.status = status.status;
                    resourceNode.hook = status.hook;
                    resourceNode.requiresPruning = status.requiresPruning;
                }
            } else {
                const status = appSetStatusByKey.get(nodeKey(node));
                if (status && status.health) {
                    resourceNode.health = {
                        status: status.health.status as models.HealthStatusCode,
                        message: ''
                    };
                }
            }
            nodeByKey.set(treeNodeKey(node), resourceNode);
        });
    const nodes = Array.from(nodeByKey.values());
    let roots: ResourceTreeNode[] = [];
    const childrenByParentKey = new Map<string, ResourceTreeNode[]>();
    const nodesHavingChildren = new Map<string, number>();
    const childrenMap = new Map<string, ResourceTreeNode[]>();
    // How much of the graph to draw, and which parts the user has asked to see more of.
    const [visibleCap, setVisibleCap] = React.useState(DEFAULT_VISIBLE_CAP);
    const [capBucketKind, setCapBucketKind] = React.useState<string | null>(null);
    const [expandedCounts, setExpandedCounts] = React.useState<{[key: string]: number}>({});
    const [showAllKindChips, setShowAllKindChips] = React.useState(false);
    // Allowance for one marker: its own expanded count if it has been clicked, else the default.
    const allowanceFor = (key: string, fallback: number) => expandedCounts[key] ?? fallback;
    const filtersRef = React.useRef(props.filters);
    const filteredGraphRef = React.useRef<any[]>([]);
    const filteredNodes: any[] = [];

    React.useEffect(() => {
        if (props.filters !== filtersRef.current) {
            filtersRef.current = props.filters;
            props.setTreeFilterGraph(filteredGraphRef.current);
            filteredGraphRef.current = filteredNodes;
        }
    }, [props.filters]);
    const {podGroupCount, userMsgs, updateUsrHelpTipMsgs, setShowCompactNodes} = props;
    const podCount = nodes.filter(node => node.kind === 'Pod').length;
    const showPodGroupByStatus = props.tree.nodes.filter((rNode: ResourceTreeNode) => rNode.kind === 'Pod').length >= props.podGroupCount;

    React.useEffect(() => {
        if (podCount > podGroupCount) {
            const userMsg = getUsrMsgKeyToDisplay(appNode.name, 'groupNodes', userMsgs);
            updateUsrHelpTipMsgs(userMsg);
            if (!userMsg.display) {
                setShowCompactNodes(true);
            }
        }
    }, [podCount]);

    function filterGraph(
        app: models.AbstractApplication,
        filteredIndicatorParent: string,
        graphNodesFilter: dagre.graphlib.Graph<{[key: string]: any}>,
        predicate: (node: ResourceTreeNode) => boolean
    ) {
        const appKey = appNodeKey(app);
        const filteredNodeIds: string[] = [];
        graphNodesFilter.nodes().forEach(nodeId => {
            const node: ResourceTreeNode = graphNodesFilter.node(nodeId) as any;
            const parentIds = graphNodesFilter.predecessors(nodeId);

            const shouldKeepNode = () => {
                //case for podgroup in group node view
                if (node.podGroup) {
                    return predicate(node) || node.podGroup.pods.some(pod => predicate({...node, kind: 'Pod', name: pod.name}));
                }
                return predicate(node);
            };

            if (node.root != null && !shouldKeepNode() && appKey !== nodeId) {
                const childIds = graphNodesFilter.successors(nodeId);
                graphNodesFilter.removeNode(nodeId);
                filteredNodeIds.push(nodeId);
                childIds.forEach((childId: any) => {
                    parentIds.forEach((parentId: any) => {
                        graphNodesFilter.setEdge(parentId, childId);
                    });
                });
            } else {
                if (node.root != null) filteredNodes.push(node);
            }
        });

        if (filteredNodeIds.length) {
            graphNodesFilter.setNode(FILTERED_INDICATOR_NODE, {
                height: NODE_HEIGHT,
                width: NODE_WIDTH,
                count: filteredNodeIds.length,
                type: NODE_TYPES.filteredIndicator
            });
            graphNodesFilter.setEdge(filteredIndicatorParent, FILTERED_INDICATOR_NODE);
        }
    }

    // Helper to check if edge should be reversed for correct traffic flow visualization
    // Gateway API routes reference Gateway via parentRefs, but traffic flows Gateway -> Route
    const gatewayAPIRouteKinds = new Set(['HTTPRoute', 'GRPCRoute', 'TCPRoute', 'TLSRoute', 'UDPRoute']);
    const shouldReverseEdge = (source: ResourceTreeNode, target: ResourceTreeNode): boolean =>
        source.group === 'gateway.networking.k8s.io' && gatewayAPIRouteKinds.has(source.kind) && target.group === 'gateway.networking.k8s.io' && target.kind === 'Gateway';

    if (props.useNetworkingHierarchy) {
        // Network view
        const hasParents = new Set<string>();
        const networkNodes = nodes.filter(node => node.networkingInfo);
        const hiddenNodes: ResourceTreeNode[] = [];
        networkNodes.forEach(parent => {
            findNetworkTargets(networkNodes, parent.networkingInfo).forEach(child => {
                // For HTTPRoute -> Gateway edges, reverse the relationship
                // so Gateway appears as the parent (traffic flows Gateway -> HTTPRoute -> Service)
                const reverseEdge = shouldReverseEdge(parent, child);
                const actualParent = reverseEdge ? child : parent;
                const actualChild = reverseEdge ? parent : child;

                const children = childrenByParentKey.get(treeNodeKey(actualParent)) || [];
                hasParents.add(treeNodeKey(actualChild));
                const parentId = actualParent.uid;
                if (nodesHavingChildren.has(parentId)) {
                    nodesHavingChildren.set(parentId, nodesHavingChildren.get(parentId) + children.length);
                } else {
                    nodesHavingChildren.set(parentId, 1);
                }
                if (actualChild.kind !== 'Pod' || !props.showCompactNodes) {
                    if (props.getNodeExpansion(parentId)) {
                        hasParents.add(treeNodeKey(actualChild));
                        children.push(actualChild);
                        childrenByParentKey.set(treeNodeKey(actualParent), children);
                    } else {
                        hiddenNodes.push(actualChild);
                    }
                } else {
                    processPodGroup(actualParent, actualChild, props);
                }
            });
        });
        roots = networkNodes.filter(node => !hasParents.has(treeNodeKey(node)));
        roots = roots.reduce((acc, curr) => {
            if (hiddenNodes.indexOf(curr) < 0) {
                acc.push(curr);
            }
            return acc;
        }, []);
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
                const colorByService = new Map<string, string>();
                (childrenByParentKey.get(treeNodeKey(root)) || []).forEach((child, i) => colorByService.set(treeNodeKey(child), TRAFFIC_COLORS[i % TRAFFIC_COLORS.length]));
                // The root goes through the budget like everything else, and before anything is hung
                // off it. Placing it directly meant the budget only ever refused children, so an
                // application with many ingress facing resources quietly lost most of its graph; and
                // an edge to a refused node leaves a dimensionless placeholder that dagre throws on.
                if (!processNode(root, root, [colorsBySource.get(treeNodeKey(root))])) {
                    return;
                }
                (childrenByParentKey.get(treeNodeKey(root)) || []).forEach(child => {
                    if (!graph.hasNode(treeNodeKey(child))) {
                        return;
                    }
                    // Draw edge if nodes are in same namespace OR if parent is cluster-scoped (no namespace)
                    if (root.namespace === child.namespace || !root.namespace) {
                        graph.setEdge(treeNodeKey(root), treeNodeKey(child), {colors: [colorByService.get(treeNodeKey(child))]});
                    }
                });
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
                if (processNode(root, root, [colorsBySource.get(treeNodeKey(root))])) {
                    graph.setEdge(INTERNAL_TRAFFIC_NODE, treeNodeKey(root));
                }
            });
        }
        if (props.nodeFilter) {
            // show filtered indicator next to external traffic node is app has it otherwise next to internal traffic node
            filterGraph(props.app, externalRoots.length > 0 ? EXTERNAL_TRAFFIC_NODE : INTERNAL_TRAFFIC_NODE, graph, props.nodeFilter);
        }
    } else {
        // Tree view
        const managedKeys = isApp(props.app)
            ? new Set((props.app as models.Application).status.resources.map(nodeKey))
            : (props.app as models.ApplicationSet).status?.resources
              ? new Set((props.app as models.ApplicationSet).status.resources.map(nodeKey))
              : new Set<string>();
        const orphanedKeys = isApp(props.app) ? new Set(props.tree.orphanedNodes?.map(nodeKey)) : new Set<string>();
        const orphans: ResourceTreeNode[] = [];
        let allChildNodes: ResourceTreeNode[] = [];
        nodesHavingChildren.set(appNode.uid, 1);
        if (props.getNodeExpansion(appNode.uid)) {
            nodes.forEach(node => {
                allChildNodes = [];
                if ((node.parentRefs || []).length === 0 || managedKeys.has(nodeKey(node))) {
                    roots.push(node);
                } else {
                    if (orphanedKeys.has(nodeKey(node))) {
                        orphans.push(node);
                    }
                    node.parentRefs.forEach(parent => {
                        const parentId = treeNodeKey(parent);
                        const children = childrenByParentKey.get(parentId) || [];
                        if (nodesHavingChildren.has(parentId)) {
                            nodesHavingChildren.set(parentId, nodesHavingChildren.get(parentId) + children.length);
                        } else {
                            nodesHavingChildren.set(parentId, 1);
                        }
                        allChildNodes.push(node);
                        if (node.kind !== 'Pod' || !props.showCompactNodes) {
                            if (props.getNodeExpansion(parentId)) {
                                children.push(node);
                                childrenByParentKey.set(parentId, children);
                            }
                        } else {
                            const parentTreeNode = nodeByKey.get(parentId);
                            processPodGroup(parentTreeNode, node, props);
                        }
                        if (props.showCompactNodes) {
                            if (childrenMap.has(parentId)) {
                                childrenMap.set(parentId, childrenMap.get(parentId).concat(allChildNodes));
                            } else {
                                childrenMap.set(parentId, allChildNodes);
                            }
                        }
                    });
                }
            });
        }
        // Ranked before the budget is spent, so what survives is what needs attention rather than
        // whatever sorts first by name. Only reordered when the budget actually bites, so an
        // application that fits keeps the ordering it has always had.
        const relevanceMemo = new Map<string, number>();
        const relevanceVisiting = new Set<string>();
        const relevanceOf = (n: ResourceTreeNode) => subtreeRelevance(n, childrenByParentKey, relevanceMemo, relevanceVisiting);
        const byRelevanceThenName = (a: ResourceTreeNode, b: ResourceTreeNode) => relevanceOf(a) - relevanceOf(b) || compareNodes(a, b);
        const budgetBites = roots.length > visibleCap && !capBucketKind;

        // Past the budget the bulk kinds get a parent of their own rather than competing for one
        // shared allowance. The top level then holds the workloads, whose hierarchy is the reason this
        // is a graph at all, plus one node per remaining kind, and it stops growing with the size of
        // the application.
        const clusters = new Map<string, ResourceTreeNode[]>();
        // Drilling into a kind shows that kind's resources at the top level, which is what was asked for.
        const inBucket = capBucketKind ? roots.filter(r => r.kind === capBucketKind) : roots;
        let orderedRoots = budgetBites || capBucketKind ? [...inBucket].sort(byRelevanceThenName) : inBucket.sort(compareNodes);
        if (budgetBites) {
            const direct: ResourceTreeNode[] = [];
            orderedRoots.forEach(root => {
                // Workloads and anything with children are never folded away: burying
                // Deployment -> ReplicaSet -> Pod under a synthetic parent hides the path a user
                // follows to explain a failure.
                if (WORKLOAD_KINDS.has(root.kind) || (childrenByParentKey.get(treeNodeKey(root)) || []).length > 0) {
                    direct.push(root);
                    return;
                }
                const members = clusters.get(root.kind);
                if (members) {
                    members.push(root);
                } else {
                    clusters.set(root.kind, [root]);
                }
            });
            // A kind small enough to show outright gains nothing from a parent.
            Array.from(clusters.entries()).forEach(([kind, members]) => {
                if (members.length <= KIND_GROUP_PREVIEW) {
                    clusters.delete(kind);
                    direct.push(...members);
                }
            });
            orderedRoots = direct.sort(byRelevanceThenName);
        }

        let shownRoots = 0;
        orderedRoots.forEach(node => {
            if (processNode(node, node)) {
                shownRoots++;
                graph.setEdge(appNodeKey(props.app), treeNodeKey(node));
            }
        });

        // One node per clustered kind, each previewing the members that most need attention and
        // carrying its own marker for the rest.
        clusters.forEach((members, kind) => {
            const kindNodeId = `${KIND_GROUP_PREFIX}${kind}`;
            graph.setNode(kindNodeId, {
                height: NODE_HEIGHT,
                width: NODE_WIDTH,
                type: NODE_TYPES.kindGroup,
                kind,
                total: members.length,
                group: members[0].group
            });
            graph.setEdge(appNodeKey(props.app), kindNodeId);
            let shownHere = 0;
            const preview = allowanceFor(kindNodeId, KIND_GROUP_PREVIEW);
            [...members].sort(byRelevanceThenName).forEach(member => {
                if (shownHere >= preview) {
                    return;
                }
                if (processNode(member, member)) {
                    shownHere++;
                    graph.setEdge(kindNodeId, treeNodeKey(member));
                }
            });
            if (members.length > shownHere) {
                const moreId = `${kindNodeId}/__more__`;
                graph.setNode(moreId, {
                    height: NODE_HEIGHT,
                    width: NODE_WIDTH,
                    type: NODE_TYPES.cappedIndicator,
                    shownCount: shownHere,
                    totalCount: members.length,
                    hiddenCount: members.length - shownHere,
                    atCeiling: visibleCap >= MAX_VISIBLE_CAP,
                    hiddenStates: tallyStates([...members].sort(byRelevanceThenName).slice(shownHere)),
                    parentKey: kindNodeId
                });
                graph.setEdge(kindNodeId, moreId);
            }
        });
        orphans.sort(compareNodes).forEach(node => {
            processNode(node, node);
        });
        // Say what is missing. Truncating quietly is worse than truncating: the user cannot tell the
        // difference between "this application has 200 resources" and "we drew 200 of 14,451".
        // While kinds are clustered the members a kind is not previewing are not missing: their kind
        // node carries the total and its own marker. The card reports only roots drawn one at a time.
        if (orderedRoots.length > shownRoots || capBucketKind) {
            const kindCounts = new Map<string, number>();
            inBucket.forEach(r => kindCounts.set(r.kind, (kindCounts.get(r.kind) || 0) + 1));
            const shownByKind = new Map<string, number>();
            orderedRoots.slice(0, shownRoots).forEach(r => shownByKind.set(r.kind, (shownByKind.get(r.kind) || 0) + 1));
            graph.setNode(CAPPED_INDICATOR_NODE, {
                height: 150,
                width: 340,
                type: NODE_TYPES.cappedIndicator,
                shownCount: shownRoots,
                totalCount: orderedRoots.length,
                hiddenCount: orderedRoots.length - shownRoots,
                atCeiling: visibleCap >= MAX_VISIBLE_CAP,
                bucket: capBucketKind,
                byKind: Array.from(kindCounts, ([kind, count]) => ({kind, count, shown: shownByKind.get(kind) || 0})).sort((a, b) => b.count - a.count)
            });
            graph.setEdge(appNodeKey(props.app), CAPPED_INDICATOR_NODE);
        }
        graph.setNode(appNodeKey(props.app), {...appNode, width: NODE_WIDTH, height: NODE_HEIGHT});
        const appSetKey = appSetNode ? nodeKey({group: 'argoproj.io', kind: 'ApplicationSet', name: appSetRef.name, namespace: props.app.metadata.namespace}) : null;
        if (appSetKey) {
            graph.setNode(appSetKey, {...appSetNode, width: NODE_WIDTH, height: NODE_HEIGHT});
            graph.setEdge(appSetKey, appNodeKey(props.app));
        }
        if (props.nodeFilter) {
            filterGraph(props.app, appSetKey || appNodeKey(props.app), graph, props.nodeFilter);
        }
        if (props.showCompactNodes) {
            const kindCounts = new Map<string, number>();
            inBucket.forEach(r => kindCounts.set(r.kind, (kindCounts.get(r.kind) || 0) + 1));
            groupNodes(nodes, graph, kindCounts, appNodeKey(props.app));
        }
    }

    function setPodGroupNode(node: ResourceTreeNode, root: ResourceTreeNode) {
        const numberOfRows = getPodGroupNumberOfRows(node.podGroup?.pods, showPodGroupByStatus);
        graph.setNode(treeNodeKey(node), {
            ...node,
            type: NODE_TYPES.podGroup,
            width: NODE_WIDTH,
            height: POD_NODE_HEIGHT + POD_GROUP_ROW_HEIGHT * numberOfRows,
            root
        });
    }

    // Returns whether the node was drawn. Callers must only draw an edge to it when it was: an edge to
    // a node the budget refused leaves a dimensionless placeholder behind, which dagre.layout throws on.
    function processNode(node: ResourceTreeNode, root: ResourceTreeNode, colors?: string[]): boolean {
        if (renderState.drawn >= visibleCap) {
            renderState.skipped++;
            return false;
        }
        renderState.drawn++;
        if (props.showCompactNodes && node.podGroup) {
            setPodGroupNode(node, root);
        } else {
            graph.setNode(treeNodeKey(node), {...node, width: NODE_WIDTH, height: NODE_HEIGHT, root});
        }
        // Capped per parent as well, so one very wide subtree cannot consume the whole budget and
        // leave every other branch undrawn.
        let shownChildren = 0;
        let hiddenChildren = 0;
        const hiddenChildNodes: ResourceTreeNode[] = [];
        // Copied before sorting: the array is cached in childrenByParentKey and sort works in place.
        const orderedChildren = [...(childrenByParentKey.get(treeNodeKey(node)) || [])].sort(
            (a, b) => subtreeRelevance(a, childrenByParentKey) - subtreeRelevance(b, childrenByParentKey) || compareNodes(a, b)
        );
        orderedChildren.forEach(child => {
            if (treeNodeKey(child) === treeNodeKey(root)) {
                return;
            }
            if (shownChildren >= allowanceFor(treeNodeKey(node), MAX_CHILDREN_PER_PARENT)) {
                hiddenChildren++;
                hiddenChildNodes.push(child);
                return;
            }
            if (!processNode(child, root, colors)) {
                return;
            }
            shownChildren++;
            // Draw edge if nodes are in same namespace OR if parent is cluster-scoped (empty/undefined namespace)
            const isParentClusterScoped = !node.namespace || node.namespace === '';
            if (node.namespace === child.namespace || isParentClusterScoped) {
                graph.setEdge(treeNodeKey(node), treeNodeKey(child), {colors});
            }
        });
        if (hiddenChildren > 0) {
            const moreId = treeNodeKey(node) + '/__more__';
            graph.setNode(moreId, {
                height: NODE_HEIGHT,
                width: NODE_WIDTH,
                type: NODE_TYPES.cappedIndicator,
                shownCount: shownChildren,
                totalCount: shownChildren + hiddenChildren,
                hiddenCount: hiddenChildren,
                atCeiling: visibleCap >= MAX_VISIBLE_CAP,
                hiddenStates: tallyStates(hiddenChildNodes),
                parentKey: treeNodeKey(node)
            });
            graph.setEdge(treeNodeKey(node), moreId);
        }
        return true;
    }
    dagre.layout(graph);

    const edges: {from: string; to: string; lines: Line[]; backgroundImage?: string; color?: string; colors?: string | {[key: string]: any}}[] = [];
    const nodeOffset = new Map<string, number>();
    const reverseEdge = new Map<string, number>();
    graph.edges().forEach(edgeInfo => {
        const edge = graph.edge(edgeInfo);
        if (edge.points.length > 1) {
            if (!reverseEdge.has(edgeInfo.w)) {
                reverseEdge.set(edgeInfo.w, 1);
            } else {
                reverseEdge.set(edgeInfo.w, reverseEdge.get(edgeInfo.w) + 1);
            }
            if (!nodeOffset.has(edgeInfo.v)) {
                nodeOffset.set(edgeInfo.v, reverseEdge.get(edgeInfo.w) - 1);
            }
        }
    });
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
            const startNode = graph.node(edgeInfo.v);
            const endNode = graph.node(edgeInfo.w);
            const offset = nodeOffset.get(edgeInfo.v);
            let startNodeRight = props.useNetworkingHierarchy ? 162 : 142;
            const endNodeLeft = 140;
            let spaceForExpansionIcon = 0;
            if (edgeInfo.v.startsWith(EXTERNAL_TRAFFIC_NODE) && !edgeInfo.v.startsWith(EXTERNAL_TRAFFIC_NODE + ':')) {
                lines.push({x1: startNode.x + 10, y1: startNode.y, x2: endNode.x - endNodeLeft, y2: endNode.y});
            } else {
                if (edgeInfo.v.startsWith(EXTERNAL_TRAFFIC_NODE + ':')) {
                    startNodeRight = 152;
                    spaceForExpansionIcon = 5;
                }
                const len = reverseEdge.get(edgeInfo.w) + 1;
                const yEnd = endNode.y - endNode.height / 2 + (endNode.height / len + (endNode.height / len) * offset);
                const firstBend =
                    spaceForExpansionIcon +
                    startNode.x +
                    startNodeRight +
                    (endNode.x - startNode.x - startNodeRight - endNodeLeft) / len +
                    ((endNode.x - startNode.x - startNodeRight - endNodeLeft) / len) * offset;
                lines.push({x1: startNode.x + startNodeRight, y1: startNode.y, x2: firstBend, y2: startNode.y});
                if (startNode.y - yEnd >= 1 || yEnd - startNode.y >= 1) {
                    lines.push({x1: firstBend, y1: startNode.y, x2: firstBend, y2: yEnd});
                }
                lines.push({x1: firstBend, y1: yEnd, x2: endNode.x - endNodeLeft, y2: yEnd});
            }
        }
        edges.push({from: edgeInfo.v, to: edgeInfo.w, lines, backgroundImage, colors: [{colors}]});
    });
    const graphNodes = graph.nodes();
    const size = getGraphSize(graphNodes.map(id => graph.node(id)));

    const resourceTreeRef = React.useRef<HTMLDivElement | null>(null);

    const graphMoving = React.useRef({
        enable: false,
        x: 0,
        y: 0
    });

    const onGraphDragStart: React.PointerEventHandler<HTMLDivElement> = e => {
        if (e.target !== resourceTreeRef.current) {
            return;
        }

        if (!resourceTreeRef.current?.parentElement) {
            return;
        }

        graphMoving.current.enable = true;
        graphMoving.current.x = e.clientX;
        graphMoving.current.y = e.clientY;
    };

    const onGraphDragMoving: React.PointerEventHandler<HTMLDivElement> = e => {
        if (!graphMoving.current.enable) {
            return;
        }

        if (!resourceTreeRef.current?.parentElement) {
            return;
        }

        const graphContainer = resourceTreeRef.current?.parentElement;

        const currentPositionX = graphContainer.scrollLeft;
        const currentPositionY = graphContainer.scrollTop;

        const scrollLeft = currentPositionX + graphMoving.current.x - e.clientX;
        const scrollTop = currentPositionY + graphMoving.current.y - e.clientY;

        graphContainer.scrollTo(scrollLeft, scrollTop);

        graphMoving.current.x = e.clientX;
        graphMoving.current.y = e.clientY;
    };

    const onGraphDragEnd: React.PointerEventHandler<HTMLDivElement> = e => {
        if (graphMoving.current.enable) {
            graphMoving.current.enable = false;
            e.preventDefault();
        }
    };
    return (
        (graphNodes.length === 0 && (
            <EmptyState icon=' fa fa-network-wired'>
                <h4>Your application has no network resources</h4>
                <h5>Try switching to tree or list view</h5>
            </EmptyState>
        )) || (
            <div
                ref={resourceTreeRef}
                onPointerDown={onGraphDragStart}
                onPointerMove={onGraphDragMoving}
                onPointerUp={onGraphDragEnd}
                onPointerLeave={onGraphDragEnd}
                className={classNames('application-resource-tree', {'application-resource-tree--network': props.useNetworkingHierarchy})}
                style={{width: size.width + 150, height: size.height + 250, transformOrigin: '0% 4%', transform: `scale(${props.zoom})`}}>
                {graphNodes.map(key => {
                    const node = graph.node(key);
                    const nodeType = node.type;
                    switch (nodeType) {
                        case NODE_TYPES.kindGroup:
                            return (
                                <React.Fragment key={key}>
                                    {renderKindGroupNode(node as any, (kind: string) => {
                                        setCapBucketKind(kind);
                                        setVisibleCap(DEFAULT_VISIBLE_CAP);
                                    })}
                                </React.Fragment>
                            );
                        case NODE_TYPES.cappedIndicator:
                            return (
                                <React.Fragment key={key}>
                                    {renderCappedNode(
                                        node as any,
                                        {
                                            onLoadMore: () => setVisibleCap(cap => Math.min(cap + EXPAND_STEP, MAX_VISIBLE_CAP)),
                                            onSelectKind: (kind: string) => {
                                                setCapBucketKind(kind);
                                                setVisibleCap(DEFAULT_VISIBLE_CAP);
                                            },
                                            onClearBucket: () => {
                                                setCapBucketKind(null);
                                                setVisibleCap(DEFAULT_VISIBLE_CAP);
                                            },
                                            onExpandParent: (parentKey: string, shownNow: number) => {
                                                setExpandedCounts(prev => ({...prev, [parentKey]: (prev[parentKey] ?? shownNow) + EXPAND_STEP}));
                                                // The budget is global, so raising one marker's allowance
                                                // without raising it means the extra nodes are refused.
                                                setVisibleCap(cap => Math.min(cap + EXPAND_STEP, MAX_VISIBLE_CAP));
                                            },
                                            onShowAllKindChips: () => setShowAllKindChips(true)
                                        },
                                        showAllKindChips
                                    )}
                                </React.Fragment>
                            );
                        case NODE_TYPES.filteredIndicator:
                            return <React.Fragment key={key}>{renderFilteredNode(node as any, props.onClearFilter)}</React.Fragment>;
                        case NODE_TYPES.externalTraffic:
                            return <React.Fragment key={key}>{renderTrafficNode(node)}</React.Fragment>;
                        case NODE_TYPES.internalTraffic:
                            return null;
                        case NODE_TYPES.externalLoadBalancer:
                            return <React.Fragment key={key}>{renderLoadBalancerNode(node as any)}</React.Fragment>;
                        case NODE_TYPES.groupedNodes:
                            return <React.Fragment key={key}>{renderGroupedNodes(props, node as any, nodes)}</React.Fragment>;
                        case NODE_TYPES.podGroup:
                            return <React.Fragment key={key}>{renderPodGroup(props, node as ResourceTreeNode & dagre.Node, childrenMap, showPodGroupByStatus)}</React.Fragment>;
                        default:
                            return <React.Fragment key={key}>{renderResourceNode(props, node as ResourceTreeNode & dagre.Node, nodesHavingChildren)}</React.Fragment>;
                    }
                })}
                {edges.map(edge => (
                    <div key={`${edge.from}-${edge.to}`} className='application-resource-tree__edge'>
                        {edge.lines.map((line, i) => {
                            const distance = Math.sqrt(Math.pow(line.x1 - line.x2, 2) + Math.pow(line.y1 - line.y2, 2));
                            const xMid = (line.x1 + line.x2) / 2;
                            const yMid = (line.y1 + line.y2) / 2;
                            const angle = (Math.atan2(line.y1 - line.y2, line.x1 - line.x2) * 180) / Math.PI;
                            const lastLine = i === edge.lines.length - 1 ? line : null;
                            let arrowColor = null;
                            if (edge.colors) {
                                if (Array.isArray(edge.colors)) {
                                    const firstColor = edge.colors[0];
                                    if (firstColor.colors) {
                                        arrowColor = firstColor.colors;
                                    }
                                }
                            }
                            return (
                                <div
                                    className='application-resource-tree__line'
                                    key={i}
                                    style={{
                                        width: distance,
                                        left: xMid - distance / 2,
                                        top: yMid,
                                        backgroundImage: edge.backgroundImage,
                                        transform: props.useNetworkingHierarchy ? `translate(140px, 35px) rotate(${angle}deg)` : `translate(150px, 35px) rotate(${angle}deg)`
                                    }}>
                                    {lastLine && props.useNetworkingHierarchy && <ArrowConnector color={arrowColor} left={xMid + distance / 2} top={yMid} angle={angle} />}
                                </div>
                            );
                        })}
                    </div>
                ))}
            </div>
        )
    );
};
