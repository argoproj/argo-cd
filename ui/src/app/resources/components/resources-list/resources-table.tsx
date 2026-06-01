import {DropDown, Tooltip} from 'argo-ui';
import * as React from 'react';
import classNames from 'classnames';
import {Key, KeybindingContext, useNav} from 'argo-ui/v2';
import {Cluster} from '../../../shared/components';
import {Consumer, Context, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {HealthPriority, SyncPriority, SyncStatusCode} from '../../../shared/models';
import {ResourceIcon} from '../../../applications/components/resource-icon';
import {ResourceLabel} from '../../../applications/components/resource-label';
import * as AppUtils from '../../../applications/components/utils';
import {getManagingApplicationUrl, openResourceDetails} from '../utils';
import '../../../applications/components/application-details/application-resource-list.scss';
import './resources-table.scss';
import {TruncatedTextTooltip, useTruncatedElement} from './truncated-text-tooltip';

type SortKey = 'name' | 'group-kind' | 'namespace' | 'cluster' | 'application' | 'status';

export const ResourcesTable = (props: {resources: models.Resource[]; onOpenDetails?: (resource: models.Resource) => void}) => {
    const [selectedResource, navResource, reset] = useNav(props.resources.length);
    const ctxh = React.useContext(Context);
    const [sortConfig, setSortConfig] = React.useState<{key: SortKey; direction: 'asc' | 'desc'}>({key: 'name', direction: 'asc'});

    const {useKeybinding} = React.useContext(KeybindingContext);

    const handleSort = (key: SortKey) => {
        setSortConfig(prev => (prev.key !== key ? {key, direction: 'asc'} : {key, direction: prev.direction === 'asc' ? 'desc' : 'asc'}));
    };

    const getSortArrow = (key: SortKey) => {
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
        const items = [...props.resources];
        items.sort((a, b) => {
            let compare = 0;
            switch (sortConfig.key) {
                case 'name':
                    compare = a.name.localeCompare(b.name);
                    break;
                case 'group-kind': {
                    const ga = [a.group, a.kind].filter(Boolean).join('/');
                    const gb = [b.group, b.kind].filter(Boolean).join('/');
                    compare = ga.localeCompare(gb);
                    break;
                }
                case 'namespace':
                    compare = (a.namespace || '').localeCompare(b.namespace || '');
                    break;
                case 'cluster': {
                    const ca = a.clusterName || a.clusterServer || '';
                    const cb = b.clusterName || b.clusterServer || '';
                    compare = ca.localeCompare(cb);
                    break;
                }
                case 'application':
                    compare = (a.appName || '').localeCompare(b.appName || '');
                    break;
                case 'status': {
                    const healthA = a.health?.status ?? 'Unknown';
                    const healthB = b.health?.status ?? 'Unknown';
                    const syncA = (a.status as SyncStatusCode) ?? 'Unknown';
                    const syncB = (b.status as SyncStatusCode) ?? 'Unknown';
                    compare = HealthPriority[healthA] - HealthPriority[healthB];
                    if (compare === 0) {
                        compare = SyncPriority[syncA] - SyncPriority[syncB];
                    }
                    break;
                }
            }
            return sortConfig.direction === 'asc' ? compare : -compare;
        });
        return items;
    }, [props.resources, sortConfig]);

    useKeybinding({keys: Key.DOWN, action: () => navResource(1)});
    useKeybinding({keys: Key.UP, action: () => navResource(-1)});
    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            reset();
            return selectedResource > -1 ? true : false;
        }
    });
    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedResource > -1 && selectedResource < sortedResources.length) {
                const resource = sortedResources[selectedResource];
                if (props.onOpenDetails) {
                    props.onOpenDetails(resource);
                } else {
                    openResourceDetails(ctxh, resource);
                }
                return true;
            }
            return false;
        }
    });

    const openDetails = (ctx: ContextApis, resource: models.Resource) => {
        if (props.onOpenDetails) {
            props.onOpenDetails(resource);
        } else {
            openResourceDetails(ctx, resource);
        }
    };

    const navigateToApplication = (ctx: ContextApis, resource: models.Resource, e?: React.MouseEvent) => {
        ctx.navigation.goto(getManagingApplicationUrl(resource.appName, resource.appNamespace), {}, e ? {event: e} : undefined);
    };

    return (
        <Consumer>
            {ctx => (
                <div className='application-details resources-table'>
                            <div className='argo-table-list argo-table-list--clickable'>
                                <div className='argo-table-list__head'>
                                    <div className='row'>
                                        <div className='columns resources-table__col-icon' />
                                        <div
                                            className='columns resources-table__col-group-kind resources-table__head-col'
                                            onClick={() => handleSort('group-kind')}
                                            style={{cursor: 'pointer'}}
                                            title='GROUP/KIND'>
                                            <span className='resources-table__head-text'>
                                                GROUP/KIND {getSortArrow('group-kind')}
                                            </span>
                                        </div>
                                        <div
                                            className='columns resources-table__col-namespace resources-table__head-col'
                                            onClick={() => handleSort('namespace')}
                                            style={{cursor: 'pointer'}}
                                            title='NAMESPACE'>
                                            <span className='resources-table__head-text'>
                                                NAMESPACE {getSortArrow('namespace')}
                                            </span>
                                        </div>
                                        <div
                                            className='columns resources-table__col-name resources-table__head-col'
                                            onClick={() => handleSort('name')}
                                            style={{cursor: 'pointer'}}
                                            title='NAME'>
                                            <span className='resources-table__head-text'>NAME {getSortArrow('name')}</span>
                                        </div>
                                        <div
                                            className='columns resources-table__col-cluster resources-table__head-col'
                                            onClick={() => handleSort('cluster')}
                                            style={{cursor: 'pointer'}}
                                            title='CLUSTER'>
                                            <span className='resources-table__head-text'>CLUSTER {getSortArrow('cluster')}</span>
                                        </div>
                                        <div
                                            className='columns resources-table__col-application resources-table__head-col'
                                            onClick={() => handleSort('application')}
                                            style={{cursor: 'pointer'}}
                                            title='Application (opens application page)'>
                                            <span className='resources-table__head-text resources-table__head-text--application'>
                                                <span className='resources-table__head-label'>APPLICATION</span>
                                                <i className='fa fa-external-link-alt resources-table__head-external-icon' aria-hidden={true} />
                                                {getSortArrow('application')}
                                            </span>
                                        </div>
                                        <div
                                            className='columns resources-table__status-col resources-table__head-col'
                                            onClick={() => handleSort('status')}
                                            style={{cursor: 'pointer'}}
                                            title='STATUS'>
                                            <span className='resources-table__head-text'>STATUS {getSortArrow('status')}</span>
                                        </div>
                                    </div>
                                </div>
                                {sortedResources.map((resource, i) => {
                                    const groupKind = [resource.group, resource.kind].filter(Boolean).join('/');
                                    return (
                                        <ResourceTableRow
                                            key={`${resource.appProject}/${resource.appName}/${resource.name}/${resource.namespace}/${resource.kind}/${resource.group}/${resource.version}`}
                                            resource={resource}
                                            groupKind={groupKind}
                                            index={i}
                                            selectedResource={selectedResource}
                                            ctx={ctx}
                                            openDetails={openDetails}
                                            navigateToApplication={navigateToApplication}
                                        />
                                    );
                                })}
                            </div>
                </div>
            )}
        </Consumer>
    );
};

const ResourceTableRow = (props: {
    resource: models.Resource;
    groupKind: string;
    index: number;
    selectedResource: number;
    ctx: ContextApis;
    openDetails: (ctx: ContextApis, resource: models.Resource) => void;
    navigateToApplication: (ctx: ContextApis, resource: models.Resource, e?: React.MouseEvent) => void;
}) => {
    const {resource, groupKind, index: i, selectedResource, ctx, openDetails, navigateToApplication} = props;
    const appLinkTruncation = useTruncatedElement<HTMLButtonElement>(resource.appName ?? '');

    return (
        <div
            className={classNames('argo-table-list__row', {
                                                'application-resource-tree__node--orphaned': resource.orphaned,
                                                'resources-table__row--selected': selectedResource === i
                                            })}
                                            onClick={e => openDetails(ctx, resource)}>
                                            <div className='row'>
                                                <div className='columns resources-table__col-icon'>
                                                    <div className='application-details__resource-icon'>
                                                        <ResourceIcon group={resource.group} kind={resource.kind} />
                                                        <div className='resources-table__kind-label'>{ResourceLabel({kind: resource.kind})}</div>
                                                    </div>
                                                </div>
                                                <div className='columns resources-table__col-group-kind'>
                                                    <TruncatedTextTooltip content={groupKind} className='application-details__item_text resources-table__tooltip-anchor' />
                                                </div>
                                                <div className='columns resources-table__col-namespace'>
                                                    <TruncatedTextTooltip content={resource.namespace} className='resources-table__tooltip-anchor' />
                                                </div>
                                                <div className='columns resources-table__col-name application-details__item'>
                                                    <TruncatedTextTooltip content={resource.name} className='application-details__item_text resources-table__tooltip-anchor' />
                                                </div>
                                                <div className='columns resources-table__col-cluster'>
                                                    <TruncatedTextTooltip
                                                        content={resource.clusterName || resource.clusterServer || ''}
                                                        className='resources-table__cell-text resources-table__tooltip-anchor'>
                                                        <Cluster server={resource.clusterServer} name={resource.clusterName} />
                                                    </TruncatedTextTooltip>
                                                </div>
                                                <div className='columns resources-table__col-application' onClick={e => e.stopPropagation()}>
                                                    <Tooltip content={resource.appName} enabled={!!resource.appName && appLinkTruncation.isTruncated}>
                                                        <button
                                                            ref={appLinkTruncation.ref}
                                                            type='button'
                                                            className='resources-table__application-link'
                                                            onClick={e => navigateToApplication(ctx, resource, e)}>
                                                            {resource.appName}
                                                        </button>
                                                    </Tooltip>
                                                </div>
                                                <div className='columns resources-table__status-col'>
                                                    {resource.health && (
                                                        <React.Fragment>
                                                            <AppUtils.HealthStatusIcon state={resource.health} /> {resource.health.status} &nbsp;
                                                        </React.Fragment>
                                                    )}
                                                    {resource.status && <AppUtils.ComparisonStatusIcon status={resource.status} resource={resource} label={true} />}
                                                </div>
                                            </div>
                                            <div className='application-details__node-menu resources-table__row-menu' onClick={e => e.stopPropagation()}>
                                                <DropDown
                                                    isMenu={true}
                                                    anchor={() => (
                                                        <button type='button' className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                            <i className='fa fa-ellipsis-v' />
                                                        </button>
                                                    )}>
                                                    {() => (
                                                        <ul>
                                                            <li
                                                                className='application-details__action-menu'
                                                                tabIndex={0}
                                                                onClick={e => {
                                                                    e.stopPropagation();
                                                                    openDetails(ctx, resource);
                                                                    document.body.click();
                                                                }}
                                                                onKeyDown={e => {
                                                                    if (e.key === 'Enter') {
                                                                        e.stopPropagation();
                                                                        openDetails(ctx, resource);
                                                                        document.body.click();
                                                                    }
                                                                }}>
                                                                <i className='fa fa-fw fa-info-circle' /> Details
                                                            </li>
                                                            <li
                                                                className='application-details__action-menu'
                                                                tabIndex={0}
                                                                onClick={e => {
                                                                    e.stopPropagation();
                                                                    navigateToApplication(ctx, resource);
                                                                    document.body.click();
                                                                }}
                                                                onKeyDown={e => {
                                                                    if (e.key === 'Enter') {
                                                                        e.stopPropagation();
                                                                        navigateToApplication(ctx, resource);
                                                                        document.body.click();
                                                                    }
                                                                }}>
                                                                <i className='fa fa-fw fa-external-link-alt' /> Open application
                                                            </li>
                                                        </ul>
                                                    )}
                                                </DropDown>
            </div>
        </div>
    );
};
