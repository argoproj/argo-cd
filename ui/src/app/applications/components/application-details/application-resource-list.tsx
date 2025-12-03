import {DropDown, Tooltip} from 'argo-ui';
import * as React from 'react';
import * as classNames from 'classnames';
import * as models from '../../../shared/models';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {
    ComparisonStatusIcon,
    HealthStatusIcon,
    nodeKey,
    isSameNode,
    createdOrNodeKey,
    resourceStatusToResourceNode,
    getApplicationLinkURLFromNode,
    getManagedByURLFromNode
} from '../utils';
import {AppDetailsPreferences} from '../../../shared/services';
import {Consumer} from '../../../shared/context';
import Moment from 'react-moment';
import {format} from 'date-fns';
import {HealthPriority, ResourceNode, SyncPriority, SyncStatusCode} from '../../../shared/models';
import './application-resource-list.scss';

export interface ApplicationResourceListProps {
    pref: AppDetailsPreferences;
    resources: models.ResourceStatus[];
    onNodeClick?: (fullName: string) => any;
    nodeMenu?: (node: models.ResourceNode) => React.ReactNode;
    tree?: models.ApplicationTree;
}

export const ApplicationResourceList = (props: ApplicationResourceListProps) => {
    const nodeByKey = new Map<string, models.ResourceNode>();
    props.tree?.nodes?.forEach(res => nodeByKey.set(nodeKey(res), res));

    const [sortConfig, setSortConfig] = React.useState<{key: string; direction: 'asc' | 'desc'}>({key: 'createdAt', direction: 'desc'});

    const handleSort = (key: string) => {
        setSortConfig(prevConfig => {
            if (prevConfig.key !== key) {
                return {key, direction: 'asc'};
            }
            return {key, direction: prevConfig.direction === 'asc' ? 'desc' : 'asc'};
        });
    };

    const getSortArrow = (key: string) => {
        if (sortConfig.key !== key) {
            return null;
        }

        const isAsc = sortConfig.direction === 'asc';
        const style: React.CSSProperties = {
            position: 'relative',
            top: isAsc ? '2px' : '-2px'
        };
        return (
            <span style={style}>
                <i className={isAsc ? 'fa fa-sort-up' : 'fa fa-sort-down'} />
            </span>
        );
    };

    const sortedResources = React.useMemo(() => {
        // Filter out resources that no longer exist in the tree (e.g., deleted resources)
        const resourcesToSort = props.resources.filter(res => nodeByKey.has(nodeKey(res)));
        resourcesToSort.sort((a, b) => {
            let compare = 0;
            switch (sortConfig.key) {
                case 'name':
                    compare = a.name.localeCompare(b.name);
                    break;

                case 'group-kind':
                    {
                        const groupKindA = [a.group, a.kind].filter(item => !!item).join('/');
                        const groupKindB = [b.group, b.kind].filter(item => !!item).join('/');
                        compare = groupKindA.localeCompare(groupKindB);
                    }
                    break;

                case 'syncOrder':
                    {
                        const waveA = a.syncWave ?? 0;
                        const waveB = b.syncWave ?? 0;
                        compare = waveA - waveB;
                    }
                    break;
                case 'namespace':
                    {
                        const namespaceA = a.namespace ?? '';
                        const namespaceB = b.namespace ?? '';
                        compare = namespaceA.localeCompare(namespaceB);
                    }
                    break;
                case 'createdAt':
                    {
                        compare = createdOrNodeKey(a).localeCompare(createdOrNodeKey(b), undefined, {numeric: true});
                    }
                    break;
                case 'status':
                    {
                        const healthA = a.health?.status ?? 'Unknown';
                        const healthB = b.health?.status ?? 'Unknown';
                        const syncA = (a.status as SyncStatusCode) ?? 'Unknown';
                        const syncB = (b.status as SyncStatusCode) ?? 'Unknown';

                        compare = HealthPriority[healthA] - HealthPriority[healthB];
                        if (compare === 0) {
                            compare = SyncPriority[syncA] - SyncPriority[syncB];
                        }
                    }
                    break;
            }
            return sortConfig.direction === 'asc' ? compare : -compare;
        });
        return resourcesToSort;
    }, [props.resources, sortConfig, props.tree]);

    const firstParentNode = props.resources.length > 0 && (nodeByKey.get(nodeKey(props.resources[0])) as ResourceNode)?.parentRefs?.[0];
    const isSameParent = firstParentNode && props.resources?.every(x => (nodeByKey.get(nodeKey(x)) as ResourceNode)?.parentRefs?.every(p => isSameNode(p, firstParentNode)));
    const isSameKind = props.resources?.every(x => x.group === props.resources[0].group && x.kind === props.resources[0].kind);
    const view = props.pref.view;

    const ParentRefDetails = () => {
        return isSameParent ? (
            <div className='resource-parent-node-info-title'>
                <div>Parent Node Info</div>
                <div className='resource-parent-node-info-title__label'>
                    <div>Name:</div>
                    <div>{firstParentNode.name}</div>
                </div>
                <div className='resource-parent-node-info-title__label'>
                    <div>Kind:</div>
                    <div>{firstParentNode.kind}</div>
                </div>
            </div>
        ) : (
            <div />
        );
    };
    return (
        props.resources.length > 0 && (
            <div>
                {/* Display only when the view is set to  or network */}
                {(view === 'tree' || view === 'network') && (
                    <div className='resource-details__header' style={{paddingTop: '20px'}}>
                        <ParentRefDetails />
                    </div>
                )}
                <div className='argo-table-list argo-table-list--clickable'>
                    <div className='argo-table-list__head'>
                        <div className='row'>
                            <div className='columns small-1 xxxlarge-1' />
                            <div className='columns small-2 xxxlarge-2' onClick={() => handleSort('name')} style={{cursor: 'pointer'}}>
                                NAME {getSortArrow('name')}
                            </div>
                            <div className='columns small-1 xxxlarge-1' onClick={() => handleSort('group-kind')} style={{cursor: 'pointer'}}>
                                GROUP/KIND {getSortArrow('group-kind')}
                            </div>
                            <div className='columns small-1 xxxlarge-1' onClick={() => handleSort('syncOrder')} style={{cursor: 'pointer'}}>
                                SYNC ORDER {getSortArrow('syncOrder')}
                            </div>
                            <div className='columns small-2 xxxlarge-1' onClick={() => handleSort('namespace')} style={{cursor: 'pointer'}}>
                                NAMESPACE {getSortArrow('namespace')}
                            </div>
                            {isSameKind && props.resources[0].kind === 'ReplicaSet' && <div className='columns small-1 xxxlarge-1'>REVISION</div>}
                            <div className='columns small-2 xxxlarge-2' onClick={() => handleSort('createdAt')} style={{cursor: 'pointer'}}>
                                CREATED AT {getSortArrow('createdAt')}
                            </div>
                            <div className='columns small-2 xxxlarge-1' onClick={() => handleSort('status')} style={{cursor: 'pointer'}}>
                                STATUS {getSortArrow('status')}
                            </div>
                        </div>
                    </div>
                    {sortedResources.map(res => {
                        const groupkindjoin = [res.group, res.kind].filter(item => !!item).join('/');
                        return (
                            <div
                                key={nodeKey(res)}
                                className={classNames('argo-table-list__row', {
                                    'application-resource-tree__node--orphaned': res.orphaned
                                })}
                                onClick={() => props.onNodeClick && props.onNodeClick(nodeKey(res))}>
                                <div className='row'>
                                    <div className='columns small-1 xxxlarge-1'>
                                        <div className='application-details__resource-icon'>
                                            <ResourceIcon group={res.group} kind={res.kind} />
                                            <br />
                                            <div>{ResourceLabel({kind: res.kind})}</div>
                                        </div>
                                    </div>
                                    <Tooltip content={res.name ?? ''} enabled={!!res.name}>
                                        <div className='columns small-2 xxxlarge-2 application-details__item'>
                                            <span className='application-details__item_text'>{res.name}</span>
                                            {res.kind === 'Application' && (
                                                <Consumer>
                                                    {ctx => {
                                                        // Get the node from the tree to access managed-by-url info
                                                        const node = nodeByKey.get(nodeKey(res));
                                                        const linkInfo = node
                                                            ? getApplicationLinkURLFromNode(node, ctx.baseHref)
                                                            : {url: ctx.baseHref + 'applications/' + res.namespace + '/' + res.name, isExternal: false};
                                                        const managedByURL = node ? getManagedByURLFromNode(node) : null;
                                                        return (
                                                            <span className='application-details__external_link'>
                                                                <a
                                                                    href={linkInfo.url}
                                                                    target={linkInfo.isExternal ? '_blank' : undefined}
                                                                    rel={linkInfo.isExternal ? 'noopener noreferrer' : undefined}
                                                                    onClick={e => e.stopPropagation()}
                                                                    title={managedByURL ? `Open application\nmanaged-by-url: ${managedByURL}` : 'Open application'}>
                                                                    <i className='fa fa-external-link-alt' />
                                                                </a>
                                                            </span>
                                                        );
                                                    }}
                                                </Consumer>
                                            )}
                                        </div>
                                    </Tooltip>
                                    <Tooltip content={groupkindjoin} enabled={!!groupkindjoin}>
                                        <div className='columns small-1 xxxlarge-1'>{groupkindjoin || '-'}</div>
                                    </Tooltip>
                                    <Tooltip content={res.syncWave ?? ''} enabled={!!res.syncWave}>
                                        <div className='columns small-1 xxxlarge-1'>{res.syncWave || '-'}</div>
                                    </Tooltip>
                                    <Tooltip content={res.namespace ?? ''} enabled={!!res.namespace}>
                                        <div className='columns small-2 xxxlarge-1'>{res.namespace}</div>
                                    </Tooltip>
                                    {isSameKind &&
                                        res.kind === 'ReplicaSet' &&
                                        ((nodeByKey.get(nodeKey(res)) as ResourceNode)?.info || [])
                                            .filter(tag => !tag.name.includes('Node'))
                                            .slice(0, 4)
                                            .map((tag, i) => {
                                                return (
                                                    <div key={i} className='columns small-1 xxxlarge-1'>
                                                        {tag?.value?.split(':')[1] || '-'}
                                                    </div>
                                                );
                                            })}
                                    <Tooltip content={res.createdAt ?? ''} enabled={!!res.createdAt}>
                                        <div className='columns small-2 xxxlarge-2'>
                                            {res.createdAt && (
                                                <span>
                                                    <Moment fromNow={true} ago={true}>
                                                        {res.createdAt}
                                                    </Moment>
                                                    &nbsp;ago &nbsp; {format(new Date(res.createdAt), 'MM/dd/yy')}
                                                </span>
                                            )}
                                        </div>
                                    </Tooltip>
                                    <div className='columns small-2 xxxlarge-1'>
                                        {res.health && (
                                            <React.Fragment>
                                                <HealthStatusIcon state={res.health} /> {res.health.status} &nbsp;
                                            </React.Fragment>
                                        )}
                                        {res.status && <ComparisonStatusIcon status={res.status} resource={res} label={true} />}
                                        {res.hook && <i title='Resource lifecycle hook' className='fa fa-anchor' />}
                                        {props.nodeMenu && (
                                            <div className='application-details__node-menu'>
                                                <DropDown
                                                    isMenu={true}
                                                    anchor={() => (
                                                        <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                            <i className='fa fa-ellipsis-v' />
                                                        </button>
                                                    )}>
                                                    {() => {
                                                        const node = nodeByKey.get(nodeKey(res));
                                                        if (node) {
                                                            return props.nodeMenu(node);
                                                        } else {
                                                            // For orphaned resources, create a ResourceNode-like object to prevent errors
                                                            return props.nodeMenu(resourceStatusToResourceNode(res));
                                                        }
                                                    }}
                                                </DropDown>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            </div>
                        );
                    })}
                </div>
            </div>
        )
    );
};
