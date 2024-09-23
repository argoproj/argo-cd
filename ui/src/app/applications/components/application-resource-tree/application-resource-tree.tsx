import {DropDown, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as dagre from 'dagre';
import * as React from 'react';
import Moment from 'react-moment';
import * as moment from 'moment';

import * as models from '../../../shared/models';

import {EmptyState} from '../../../shared/components';
import {AppContext, Consumer} from '../../../shared/context';
import {ApplicationURLs} from '../application-urls';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {
    BASE_COLORS,
    ComparisonStatusIcon,
    getAppOverridesCount,
    HealthStatusIcon,
    isAppNode,
    isYoungerThanXMinutes,
    NodeId,
    nodeKey,
    PodHealthIcon,
    getUsrMsgKeyToDisplay
} from '../utils';
import {NodeUpdateAnimation} from './node-update-animation';
import {PodGroup} from '../application-pod-view/pod-view';
import './application-resource-tree.scss';
import {ArrowConnector} from './arrow-connector';

function treeNodeKey(node: NodeId & {uid?: string}) {
    return node.uid || nodeKey(node);
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
    app: models.Application;
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
    setNodeExpansion: (node: string, isExpanded: boolean) => any;
    getNodeExpansion: (node: string) => boolean;
}

interface Line {
    x1: number;
    y1: number;
    x2: number;
    y2: number;
}

const NODE_WIDTH = 282;
const NODE_HEIGHT = 52;
const POD_NODE_HEIGHT = 136;
const FILTERED_INDICATOR_NODE = '__filtered_indicator__';
const EXTERNAL_TRAFFIC_NODE = '__external_traffic__';
const INTERNAL_TRAFFIC_NODE = '__internal_traffic__';
const NODE_TYPES = {
    filteredIndicator: 'filtered_indicator',
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

function groupNodes(nodes: ResourceTreeNode[], graph: dagre.graphlib.Graph) {
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
                graph.setNode(`${parentIds[0].toString()}/child/${kind}`, {
                    kind,
                    groupedNodeIds,
                    height: NODE_HEIGHT,
                    width: NODE_WIDTH,
                    count: reducedNodeIds.length,
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
            return a.localeCompare(b);
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
            nodeKey(first).localeCompare(nodeKey(second)) ||
            0
        );
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

function renderGroupedNodes(props: ApplicationResourceTreeProps, node: {count: number} & dagre.Node & ResourceTreeNode) {
    const indicators = new Array<number>();
    let count = Math.min(node.count - 1, 3);
    while (count > 0) {
        indicators.push(count--);
    }
    return (
        <React.Fragment>
            <div className='application-resource-tree__node' style={{left: node.x, top: node.y, width: node.width, height: node.height}}>
                <div className='application-resource-tree__node-kind-icon'>
                    <ResourceIcon kind={node.kind} />
                    <br />
                    <div className='application-resource-tree__node-kind'>{ResourceLabel({kind: node.kind})}</div>
                </div>
                <div
                    className='application-resource-tree__node-title application-resource-tree__direction-center-left'
                    onClick={() => props.onGroupdNodeClick && props.onGroupdNodeClick(node.groupedNodeIds)}
                    title={`Click to see details of ${node.count} collapsed ${node.kind} and doesn't contains any active pods`}>
                    {node.count} {node.kind}s
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

function renderPodGroup(props: ApplicationResourceTreeProps, id: string, node: ResourceTreeNode & dagre.Node, childMap: Map<string, ResourceTreeNode[]>) {
    const fullName = nodeKey(node);
    let comparisonStatus: models.SyncStatusCode = null;
    let healthState: models.HealthStatus = null;
    if (node.status || node.health) {
        comparisonStatus = node.status;
        healthState = node.health;
    }
    const appNode = isAppNode(node);
    const rootNode = !node.root;
    const extLinks: string[] = props.app.status.summary.externalURLs;
    const podGroupChildren = childMap.get(treeNodeKey(node));
    const nonPodChildren = podGroupChildren?.reduce((acc, child) => {
        if (child.kind !== 'Pod') {
            acc.push(child);
        }
        return acc;
    }, []);
    const childCount = nonPodChildren?.length;
    const margin = 8;
    let topExtra = 0;
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

    const showPodGroupByStatus = props.tree.nodes.filter((rNode: ResourceTreeNode) => rNode.kind === 'Pod').length >= props.podGroupCount;
    const numberOfRows = showPodGroupByStatus
        ? [podGroupHealthy, podGroupDegraded, podGroupInProgress].reduce((total, podGroupByStatus) => total + (podGroupByStatus.filter(pod => pod).length > 0 ? 1 : 0), 0)
        : Math.ceil(podGroup?.pods.length / 8);

    if (podGroup) {
        topExtra = margin + (POD_NODE_HEIGHT / 2 + 30 * numberOfRows) / 2;
    }

    return (
        <div
            className={classNames('application-resource-tree__node', {
                'active': fullName === props.selectedNodeFullName,
                'application-resource-tree__node--orphaned': node.orphaned
            })}
            title={describeNode(node)}
            style={{
                left: node.x,
                top: node.y - topExtra,
                width: node.width,
                height: showPodGroupByStatus ? POD_NODE_HEIGHT + 20 * numberOfRows : node.height
            }}>
            <NodeUpdateAnimation resourceVersion={node.resourceVersion} />
            <div onClick={() => props.onNodeClick && props.onNodeClick(fullName)} className={`application-resource-tree__node__top-part`}>
                <div
                    className={classNames('application-resource-tree__node-kind-icon', {
                        'application-resource-tree__node-kind-icon--big': rootNode
                    })}>
                    <ResourceIcon kind={node.kind || 'Unknown'} />
                    <br />
                    {!rootNode && <div className='application-resource-tree__node-kind'>{ResourceLabel({kind: node.kind})}</div>}
                </div>
                <div className='application-resource-tree__node-content'>
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
                                {ctx => (
                                    <a href={ctx.baseHref + 'applications/' + node.namespace + '/' + node.name} title='Open application'>
                                        <i className='fa fa-external-link-alt' />
                                    </a>
                                )}
                            </Consumer>
                        )}
                        <ApplicationURLs urls={rootNode ? extLinks : node.networkingInfo && node.networkingInfo.externalURLs} />
                    </span>
                    {childCount > 0 && (
                        <>
                            <br />
                            <div
                                style={{top: node.height / 2 - 6}}
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
                                    {(node.info || []).map(i => (
                                        <div key={i.name}>
                                            {i.name}: {i.value}
                                        </div>
                                    ))}
                                </>
                            }
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
        const val = `${tag.name}`;
        if (val === 'Status Reason') {
            if (`${tag.value}` !== 'ImagePullBackOff')
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
            const arr = `${tag.value}`.split('/');
            const title = `Number of containers in total: ${arr[1]} \nNumber of ready containers: ${arr[0]}`;
            return (
                <span className='application-resource-tree__node-label' title={`${title}`}>
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

function renderResourceNode(props: ApplicationResourceTreeProps, id: string, node: ResourceTreeNode & dagre.Node, nodesHavingChildren: Map<string, number>) {
    const fullName = nodeKey(node);
    let comparisonStatus: models.SyncStatusCode = null;
    let healthState: models.HealthStatus = null;
    if (node.status || node.health) {
        comparisonStatus = node.status;
        healthState = node.health;
    }
    const appNode = isAppNode(node);
    const rootNode = !node.root;
    const extLinks: string[] = props.app.status.summary.externalURLs;
    const childCount = nodesHavingChildren.get(node.uid);
    return (
        <div
            onClick={() => props.onNodeClick && props.onNodeClick(fullName)}
            className={classNames('application-resource-tree__node', 'application-resource-tree__node--' + node.kind.toLowerCase(), {
                'active': fullName === props.selectedNodeFullName,
                'application-resource-tree__node--orphaned': node.orphaned
            })}
            title={describeNode(node)}
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
                <ResourceIcon kind={node.kind} />
                <br />
                {!rootNode && <div className='application-resource-tree__node-kind'>{ResourceLabel({kind: node.kind})}</div>}
            </div>
            <div className='application-resource-tree__node-content'>
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
                            {ctx => (
                                <a href={ctx.baseHref + 'applications/' + node.namespace + '/' + node.name} title='Open application'>
                                    <i className='fa fa-external-link-alt' />
                                </a>
                            )}
                        </Consumer>
                    )}
                    <ApplicationURLs urls={rootNode ? extLinks : node.networkingInfo && node.networkingInfo.externalURLs} />
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
                {node.createdAt || rootNode ? (
                    <span title={`${node.kind} was created ${moment(node.createdAt).fromNow()}`}>
                        <Moment className='application-resource-tree__node-label' fromNow={true} ago={true}>
                            {node.createdAt || props.app.metadata.creationTimestamp}
                        </Moment>
                    </span>
                ) : null}
                {(node.info || [])
                    .filter(tag => !tag.name.includes('Node'))
                    .slice(0, 4)
                    .map((tag, i) => {
                        return <NodeInfoDetails tag={tag} kind={node.kind} key={i} />;
                    })}
                {(node.info || []).length > 4 && (
                    <Tooltip
                        content={
                            <>
                                {(node.info || []).map(i => (
                                    <div key={i.name}>
                                        {i.name}: {i.value}
                                    </div>
                                ))}
                            </>
                        }
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
        status: props.app.status.sync.status,
        health: props.app.status.health,
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
    const nodesHavingChildren = new Map<string, number>();
    const childrenMap = new Map<string, ResourceTreeNode[]>();
    const [filters, setFilters] = React.useState(props.filters);
    const [filteredGraph, setFilteredGraph] = React.useState([]);
    const filteredNodes: any[] = [];

    React.useEffect(() => {
        if (props.filters !== filters) {
            setFilters(props.filters);
            setFilteredGraph(filteredNodes);
            props.setTreeFilterGraph(filteredGraph);
        }
    }, [props.filters]);
    const {podGroupCount, userMsgs, updateUsrHelpTipMsgs, setShowCompactNodes} = props;
    const podCount = nodes.filter(node => node.kind === 'Pod').length;

    React.useEffect(() => {
        if (podCount > podGroupCount) {
            const userMsg = getUsrMsgKeyToDisplay(appNode.name, 'groupNodes', userMsgs);
            updateUsrHelpTipMsgs(userMsg);
            if (!userMsg.display) {
                setShowCompactNodes(true);
            }
        }
    }, [podCount]);

    function filterGraph(app: models.Application, filteredIndicatorParent: string, graphNodesFilter: dagre.graphlib.Graph, predicate: (node: ResourceTreeNode) => boolean) {
        const appKey = appNodeKey(app);
        let filtered = 0;
        graphNodesFilter.nodes().forEach(nodeId => {
            const node: ResourceTreeNode = graphNodesFilter.node(nodeId) as any;
            const parentIds = graphNodesFilter.predecessors(nodeId);
            if (node.root != null && !predicate(node) && appKey !== nodeId) {
                const childIds = graphNodesFilter.successors(nodeId);
                graphNodesFilter.removeNode(nodeId);
                filtered++;
                childIds.forEach((childId: any) => {
                    parentIds.forEach((parentId: any) => {
                        graphNodesFilter.setEdge(parentId, childId);
                    });
                });
            } else {
                if (node.root != null) filteredNodes.push(node);
            }
        });
        if (filtered) {
            graphNodesFilter.setNode(FILTERED_INDICATOR_NODE, {height: NODE_HEIGHT, width: NODE_WIDTH, count: filtered, type: NODE_TYPES.filteredIndicator});
            graphNodesFilter.setEdge(filteredIndicatorParent, FILTERED_INDICATOR_NODE);
        }
    }

    if (props.useNetworkingHierarchy) {
        // Network view
        const hasParents = new Set<string>();
        const networkNodes = nodes.filter(node => node.networkingInfo);
        const hiddenNodes: ResourceTreeNode[] = [];
        networkNodes.forEach(parent => {
            findNetworkTargets(networkNodes, parent.networkingInfo).forEach(child => {
                const children = childrenByParentKey.get(treeNodeKey(parent)) || [];
                hasParents.add(treeNodeKey(child));
                const parentId = parent.uid;
                if (nodesHavingChildren.has(parentId)) {
                    nodesHavingChildren.set(parentId, nodesHavingChildren.get(parentId) + children.length);
                } else {
                    nodesHavingChildren.set(parentId, 1);
                }
                if (child.kind !== 'Pod' || !props.showCompactNodes) {
                    if (props.getNodeExpansion(parentId)) {
                        hasParents.add(treeNodeKey(child));
                        children.push(child);
                        childrenByParentKey.set(treeNodeKey(parent), children);
                    } else {
                        hiddenNodes.push(child);
                    }
                } else {
                    processPodGroup(parent, child, props);
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
                (childrenByParentKey.get(treeNodeKey(root)) || []).sort(compareNodes).forEach(child => {
                    processNode(child, root, [colorByService.get(treeNodeKey(child))]);
                });
                if (root.podGroup && props.showCompactNodes) {
                    setPodGroupNode(root, root);
                } else {
                    graph.setNode(treeNodeKey(root), {...root, width: NODE_WIDTH, height: NODE_HEIGHT, root});
                }
                (childrenByParentKey.get(treeNodeKey(root)) || []).forEach(child => {
                    if (root.namespace === child.namespace) {
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
        const orphanedKeys = new Set(props.tree.orphanedNodes?.map(nodeKey));
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
        if (props.showCompactNodes) {
            groupNodes(nodes, graph);
        }
    }

    function setPodGroupNode(node: ResourceTreeNode, root: ResourceTreeNode) {
        const numberOfRows = Math.ceil(node.podGroup.pods.length / 8);
        graph.setNode(treeNodeKey(node), {...node, type: NODE_TYPES.podGroup, width: NODE_WIDTH, height: POD_NODE_HEIGHT + 30 * numberOfRows, root});
    }

    function processNode(node: ResourceTreeNode, root: ResourceTreeNode, colors?: string[]) {
        if (props.showCompactNodes && node.podGroup) {
            setPodGroupNode(node, root);
        } else {
            graph.setNode(treeNodeKey(node), {...node, width: NODE_WIDTH, height: NODE_HEIGHT, root});
        }
        (childrenByParentKey.get(treeNodeKey(node)) || []).sort(compareNodes).forEach(child => {
            if (treeNodeKey(child) === treeNodeKey(root)) {
                return;
            }
            if (node.namespace === child.namespace) {
                graph.setEdge(treeNodeKey(node), treeNodeKey(child), {colors});
            }
            processNode(child, root, colors);
        });
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

    const resourceTreeRef = React.useRef<HTMLDivElement>();

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
                style={{width: size.width + 150, height: size.height + 250, transformOrigin: '0% 0%', transform: `scale(${props.zoom})`}}>
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
                        case NODE_TYPES.groupedNodes:
                            return <React.Fragment key={key}>{renderGroupedNodes(props, node as any)}</React.Fragment>;
                        case NODE_TYPES.podGroup:
                            return <React.Fragment key={key}>{renderPodGroup(props, key, node as ResourceTreeNode & dagre.Node, childrenMap)}</React.Fragment>;
                        default:
                            return <React.Fragment key={key}>{renderResourceNode(props, key, node as ResourceTreeNode & dagre.Node, nodesHavingChildren)}</React.Fragment>;
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
