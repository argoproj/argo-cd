import {DropDown, Tooltip} from 'argo-ui';
import * as React from 'react';
import classNames from 'classnames';
import {take} from 'rxjs/operators';
import * as models from '../../../shared/models';
import {DataLoader} from '../../../shared/components';
import {services} from '../../../shared/services';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {
    ComparisonStatusIcon,
    HealthStatusIcon,
    nodeKey,
    isSameNode,
    resourceStatusToResourceNode,
    getApplicationLinkURLFromNode,
    getManagedByURLFromNode,
    MANAGED_BY_URL_INVALID_TEXT,
    MANAGED_BY_URL_INVALID_COLOR
} from '../utils';
import {AppDetailsPreferences} from '../../../shared/services';
import {Consumer} from '../../../shared/context';
import Moment from 'react-moment';
import {format} from 'date-fns';
import {ResourceNode} from '../../../shared/models';
import {isValidManagedByURL} from '../../../shared/utils';
import {APPLICATION_RESOURCE_SORT_KEY_TO_TITLE, APPLICATION_RESOURCE_SORT_TITLE_TO_KEY, ApplicationResourceSortKey} from './application-resource-sort';
import './application-resource-list.scss';

function handleSort(sortPreferencesKey: string, key: ApplicationResourceSortKey, onSortChange?: () => void) {
    services.viewPreferences
        .getPreferences()
        .pipe(take(1))
        .subscribe(pref => {
            if (!pref.sortOptions) {
                pref.sortOptions = {};
            }
            if (!pref.sortDirections) {
                pref.sortDirections = {};
            }
            const title = APPLICATION_RESOURCE_SORT_KEY_TO_TITLE[key];
            const currentTitle = pref.sortOptions[sortPreferencesKey] || 'Created At';
            const currentDir = pref.sortDirections[sortPreferencesKey] || 'desc';
            pref.sortOptions[sortPreferencesKey] = title;
            pref.sortDirections[sortPreferencesKey] = currentTitle === title && currentDir === 'asc' ? 'desc' : 'asc';
            services.viewPreferences.updatePreferences(pref);
            onSortChange?.();
        });
}

export interface ApplicationResourceListProps {
    pref: AppDetailsPreferences;
    resources: models.ResourceStatus[];
    sortPreferencesKey: string;
    onSortChange?: () => void;
    onNodeClick?: (fullName: string) => any;
    nodeMenu?: (node: models.ResourceNode) => React.ReactNode;
    tree?: models.ApplicationTree;
    selectedNodeFullName?: string;
}

export const ApplicationResourceList = (props: ApplicationResourceListProps) => {
    const nodeByKey = new Map<string, models.ResourceNode>();
    props.tree?.nodes?.forEach(res => nodeByKey.set(nodeKey(res), res));
    const selectedRowRef = React.useRef<HTMLDivElement | null>(null);

    React.useEffect(() => {
        if (props.selectedNodeFullName && selectedRowRef.current) {
            selectedRowRef.current.scrollIntoView({block: 'nearest', behavior: 'smooth'});
        }
    }, [props.selectedNodeFullName, props.resources]);

    const getSortArrow = (activeKey: ApplicationResourceSortKey, direction: 'asc' | 'desc', key: ApplicationResourceSortKey) => {
        if (activeKey !== key) {
            return null;
        }

        const isAsc = direction === 'asc';
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

    const firstParentNode = props.resources.length > 0 && (nodeByKey.get(nodeKey(props.resources[0])) as ResourceNode)?.parentRefs?.[0];
    const isSameParent = firstParentNode && props.resources?.every(x => (nodeByKey.get(nodeKey(x)) as ResourceNode)?.parentRefs?.every(p => isSameNode(p, firstParentNode)));
    const isSameKind = props.resources?.every(x => x.group === props.resources[0].group && x.kind === props.resources[0].kind);
    const showRevision = isSameKind && props.resources[0]?.kind === 'ReplicaSet';
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
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        const sortTitle = pref.sortOptions?.[props.sortPreferencesKey] || 'Created At';
                        const sortDirection = pref.sortDirections?.[props.sortPreferencesKey] || 'desc';
                        const activeSortKey = APPLICATION_RESOURCE_SORT_TITLE_TO_KEY[sortTitle] || 'createdAt';

                        return (
                            <div
                                className={classNames('argo-table-list argo-table-list--clickable application-resource-list', {
                                    'application-resource-list--with-revision': showRevision
                                })}>
                                <div className='argo-table-list__head'>
                                    <div className='row'>
                                        <div className='application-resource-list__col-icon' />
                                        <div
                                            className='application-resource-list__col-name application-resource-list__head-col'
                                            onClick={() => handleSort(props.sortPreferencesKey, 'name', props.onSortChange)}
                                            style={{cursor: 'pointer'}}>
                                            NAME {getSortArrow(activeSortKey, sortDirection, 'name')}
                                        </div>
                                        <div
                                            className='application-resource-list__col-group-kind application-resource-list__head-col'
                                            onClick={() => handleSort(props.sortPreferencesKey, 'group-kind', props.onSortChange)}
                                            style={{cursor: 'pointer'}}>
                                            GROUP/KIND {getSortArrow(activeSortKey, sortDirection, 'group-kind')}
                                        </div>
                                        <div
                                            className='application-resource-list__col-sync-order application-resource-list__head-col'
                                            onClick={() => handleSort(props.sortPreferencesKey, 'syncOrder', props.onSortChange)}
                                            style={{cursor: 'pointer'}}>
                                            SYNC ORDER {getSortArrow(activeSortKey, sortDirection, 'syncOrder')}
                                        </div>
                                        <div
                                            className='application-resource-list__col-namespace application-resource-list__head-col'
                                            onClick={() => handleSort(props.sortPreferencesKey, 'namespace', props.onSortChange)}
                                            style={{cursor: 'pointer'}}>
                                            NAMESPACE {getSortArrow(activeSortKey, sortDirection, 'namespace')}
                                        </div>
                                        {showRevision && <div className='application-resource-list__col-revision application-resource-list__head-col'>REVISION</div>}
                                        <div
                                            className='application-resource-list__col-created-at application-resource-list__head-col'
                                            onClick={() => handleSort(props.sortPreferencesKey, 'createdAt', props.onSortChange)}
                                            style={{cursor: 'pointer'}}>
                                            CREATED AT {getSortArrow(activeSortKey, sortDirection, 'createdAt')}
                                        </div>
                                        <div
                                            className='application-resource-list__col-status application-resource-list__head-col'
                                            onClick={() => handleSort(props.sortPreferencesKey, 'status', props.onSortChange)}
                                            style={{cursor: 'pointer'}}>
                                            STATUS {getSortArrow(activeSortKey, sortDirection, 'status')}
                                        </div>
                                    </div>
                                </div>
                                {props.resources.map(res => {
                                    const groupkindjoin = [res.group, res.kind].filter(item => !!item).join('/');
                                    const resKey = nodeKey(res);
                                    const isSelected = props.selectedNodeFullName === resKey;
                                    return (
                                        <div
                                            key={isSelected ? `${resKey}-highlighted` : resKey}
                                            ref={isSelected ? selectedRowRef : undefined}
                                            className={classNames('argo-table-list__row', {
                                                'application-resource-list__row--selected': isSelected,
                                                'application-resource-tree__node--orphaned': res.orphaned
                                            })}
                                            onClick={() => props.onNodeClick && props.onNodeClick(resKey)}>
                                            <div className='row'>
                                                <div className='application-resource-list__col-icon'>
                                                    <div className='application-details__resource-icon'>
                                                        <ResourceIcon group={res.group} kind={res.kind} />
                                                        <br />
                                                        <div>{ResourceLabel({kind: res.kind})}</div>
                                                    </div>
                                                </div>
                                                <Tooltip content={res.name} enabled={!!res.name}>
                                                    <div className='application-resource-list__col-name application-details__item'>
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
                                                                    const managedByURLInvalid = !!managedByURL && !isValidManagedByURL(managedByURL);
                                                                    if (managedByURLInvalid) {
                                                                        return (
                                                                            <span
                                                                                className='application-details__external_link'
                                                                                style={{cursor: 'not-allowed', display: 'inline-flex', alignItems: 'center'}}
                                                                                onClick={e => {
                                                                                    e.stopPropagation();
                                                                                }}
                                                                                title={`Open application\n${MANAGED_BY_URL_INVALID_TEXT}`}>
                                                                                <i className='fa fa-external-link-alt' style={{color: MANAGED_BY_URL_INVALID_COLOR}} />
                                                                            </span>
                                                                        );
                                                                    }
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
                                                <Tooltip content={groupkindjoin}>
                                                    <div className='application-resource-list__col-group-kind'>{groupkindjoin}</div>
                                                </Tooltip>
                                                <Tooltip content={res.syncWave} enabled={!!res.syncWave}>
                                                    <div className='application-resource-list__col-sync-order'>{res.syncWave ?? '-'}</div>
                                                </Tooltip>
                                                <Tooltip content={res.namespace} enabled={!!res.namespace}>
                                                    <div className='application-resource-list__col-namespace'>{res.namespace}</div>
                                                </Tooltip>
                                                {showRevision && (
                                                    <div className='application-resource-list__col-revision'>
                                                        {((nodeByKey.get(nodeKey(res)) as ResourceNode).info || [])
                                                            .filter(tag => !tag.name.includes('Node'))
                                                            .slice(0, 1)
                                                            .map(tag => tag?.value?.split(':')[1] || '-')
                                                            .join('') || '-'}
                                                    </div>
                                                )}
                                                <Tooltip content={res.createdAt} enabled={!!res.createdAt}>
                                                    <div className='application-resource-list__col-created-at'>
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
                                                <div className='application-resource-list__col-status'>
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
                                                                    <button
                                                                        className='argo-button argo-button--light argo-button--lg argo-button--short'
                                                                        onMouseDown={() => document.body.click()}>
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
                        );
                    }}
                </DataLoader>
            </div>
        )
    );
};
