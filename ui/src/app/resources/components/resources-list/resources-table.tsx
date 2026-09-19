import {Tooltip} from 'argo-ui';
import * as React from 'react';
import classNames from 'classnames';
import {Key, KeybindingContext, useNav} from 'argo-ui/v2';
import {take} from 'rxjs/operators';
import {ActionMenu, CellLink, Cluster, DataLoader} from '../../../shared/components';
import {Consumer, Context, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ResourceIcon} from '../../../applications/components/resource-icon';
import {ResourceLabel} from '../../../applications/components/resource-label';
import * as AppUtils from '../../../applications/components/utils';
import {
    getResourceAppLink,
    getResourceRowLink,
    navigateToManagingApplication,
    openResourceDetails,
    openResourceDetailsInApplication,
    resourceHealthState,
    resourceHealthStatus
} from '../utils';
import './resources-table.scss';
import {TruncatedTextTooltip, useTruncatedElement} from './truncated-text-tooltip';
import {RESOURCE_SORT_KEY_TO_TITLE, RESOURCE_SORT_TITLE_TO_KEY, RESOURCES_LIST_SORT_KEY, ResourceSortKey} from './resources-sort';

function handleSort(ctx: ContextApis, key: ResourceSortKey) {
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
            const title = RESOURCE_SORT_KEY_TO_TITLE[key];
            const currentTitle = pref.sortOptions[RESOURCES_LIST_SORT_KEY] || 'Name';
            const currentDir = pref.sortDirections[RESOURCES_LIST_SORT_KEY] || 'asc';
            pref.sortOptions[RESOURCES_LIST_SORT_KEY] = title;
            pref.sortDirections[RESOURCES_LIST_SORT_KEY] = currentTitle === title && currentDir === 'asc' ? 'desc' : 'asc';
            services.viewPreferences.updatePreferences(pref);
            ctx.navigation.goto('.', {page: 0}, {replace: true});
        });
}

export const ResourcesTable = (props: {resources: models.Resource[]; onOpenDetails?: (resource: models.Resource) => void}) => {
    const [selectedResource, navResource, reset] = useNav(props.resources.length);
    const ctxh = React.useContext(Context);

    const {registerKeybinding} = React.useContext(KeybindingContext);

    const getSortArrow = (activeKey: ResourceSortKey, direction: 'asc' | 'desc', key: ResourceSortKey) => {
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

    registerKeybinding({keys: Key.DOWN, action: () => navResource(1)});
    registerKeybinding({keys: Key.UP, action: () => navResource(-1)});
    registerKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            reset();
            return selectedResource > -1 ? true : false;
        }
    });
    registerKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedResource > -1 && selectedResource < props.resources.length) {
                const resource = props.resources[selectedResource];
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

    const navigateToApplication = (ctx: ContextApis, resource: models.Resource, e?: React.MouseEvent) => {
        navigateToManagingApplication(ctx, resource, e);
    };

    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        const sortTitle = pref.sortOptions?.[RESOURCES_LIST_SORT_KEY] || 'Name';
                        const sortDirection = pref.sortDirections?.[RESOURCES_LIST_SORT_KEY] || 'asc';
                        const activeSortKey = RESOURCE_SORT_TITLE_TO_KEY[sortTitle] || 'name';

                        return (
                            <div className='application-details resources-table'>
                                <div className='argo-table-list argo-table-list--clickable'>
                                    <div className='argo-table-list__head'>
                                        <div className='row'>
                                            <div
                                                className='columns small-1 large-2 resources-table__col-group-kind resources-table__head-col'
                                                onClick={() => handleSort(ctx, 'group-kind')}
                                                style={{cursor: 'pointer'}}
                                                title='GROUP/KIND'>
                                                <span className='resources-table__head-text'>GROUP/KIND {getSortArrow(activeSortKey, sortDirection, 'group-kind')}</span>
                                            </div>
                                            <div
                                                className='columns small-2 large-1 resources-table__col-namespace resources-table__head-col'
                                                onClick={() => handleSort(ctx, 'namespace')}
                                                style={{cursor: 'pointer'}}
                                                title='NAMESPACE'>
                                                <span className='resources-table__head-text'>NAMESPACE {getSortArrow(activeSortKey, sortDirection, 'namespace')}</span>
                                            </div>
                                            <div
                                                className='columns small-3 resources-table__col-name resources-table__head-col'
                                                onClick={() => handleSort(ctx, 'name')}
                                                style={{cursor: 'pointer'}}
                                                title='NAME'>
                                                <span className='resources-table__head-text'>NAME {getSortArrow(activeSortKey, sortDirection, 'name')}</span>
                                            </div>
                                            <div
                                                className='columns small-1 large-2 resources-table__col-cluster resources-table__head-col'
                                                onClick={() => handleSort(ctx, 'cluster')}
                                                style={{cursor: 'pointer'}}
                                                title='CLUSTER'>
                                                <span className='resources-table__head-text'>CLUSTER {getSortArrow(activeSortKey, sortDirection, 'cluster')}</span>
                                            </div>
                                            <div
                                                className='columns small-2 resources-table__col-application resources-table__head-col'
                                                onClick={() => handleSort(ctx, 'application')}
                                                style={{cursor: 'pointer'}}
                                                title='Application (opens application page)'>
                                                <span className='resources-table__head-text resources-table__head-text--application'>
                                                    <span className='resources-table__head-label'>APPLICATION</span>
                                                    <i className='fa fa-external-link-alt resources-table__head-external-icon' aria-hidden={true} />
                                                    {getSortArrow(activeSortKey, sortDirection, 'application')}
                                                </span>
                                            </div>
                                            <div
                                                className='columns small-3 large-2 resources-table__status-col resources-table__head-col'
                                                onClick={() => handleSort(ctx, 'status')}
                                                style={{cursor: 'pointer'}}
                                                title='STATUS'>
                                                <span className='resources-table__head-text'>STATUS {getSortArrow(activeSortKey, sortDirection, 'status')}</span>
                                            </div>
                                        </div>
                                    </div>
                                    {props.resources.map((resource, i) => {
                                        const groupKind = [resource.group, resource.kind].filter(Boolean).join('/');
                                        return (
                                            <ResourceTableRow
                                                key={`${resource.appProject}/${resource.appName}/${resource.name}/${resource.namespace}/${resource.kind}/${resource.group}/${resource.version}`}
                                                resource={resource}
                                                groupKind={groupKind}
                                                index={i}
                                                selectedResource={selectedResource}
                                                ctx={ctx}
                                                navigateToApplication={navigateToApplication}
                                            />
                                        );
                                    })}
                                </div>
                            </div>
                        );
                    }}
                </DataLoader>
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
    navigateToApplication: (ctx: ContextApis, resource: models.Resource, e?: React.MouseEvent) => void;
}) => {
    const {resource, groupKind, index: i, selectedResource, ctx, navigateToApplication} = props;
    const {ref: appLinkRef, isTruncated: appLinkIsTruncated} = useTruncatedElement<HTMLAnchorElement>(resource.appName ?? '');

    // Row navigation lives on an overlay <a> (single tab stop) plus CellLink wrappers on the cell
    // content, so middle-click / cmd-click / "open in new tab" and truncation tooltips work. The
    // Application column and the action menu are separate links/controls lifted above the overlay.
    const rowLink = getResourceRowLink(ctx, resource);
    const appLink = getResourceAppLink(ctx, resource);

    return (
        <div
            className={classNames('argo-table-list__row resources-table__row', {
                'application-resource-tree__node--orphaned': resource.orphaned,
                'resources-table__row--selected': selectedResource === i
            })}>
            <a
                className='resources-table__row__overlay-link'
                href={rowLink.href}
                onClick={rowLink.onClick}
                aria-label={[resource.kind, resource.namespace, resource.name].filter(Boolean).join(' ')}
            />
            <div className='row'>
                <div className='resources-table__col-icon'>
                    <ResourceIcon group={resource.group} kind={resource.kind} />
                    <div className='resources-table__kind'>{ResourceLabel({kind: resource.kind})}</div>
                </div>
                <div className='columns small-1 large-2 resources-table__col-group-kind'>
                    <CellLink href={rowLink.href} onClick={rowLink.onClick}>
                        <TruncatedTextTooltip content={groupKind} className='application-details__item_text resources-table__tooltip-anchor' />
                    </CellLink>
                </div>
                <div className='columns small-2 large-1 resources-table__col-namespace'>
                    <CellLink href={rowLink.href} onClick={rowLink.onClick}>
                        <TruncatedTextTooltip content={resource.namespace} className='resources-table__tooltip-anchor' />
                    </CellLink>
                </div>
                <div className='columns small-3 resources-table__col-name application-details__item'>
                    <CellLink href={rowLink.href} onClick={rowLink.onClick}>
                        <TruncatedTextTooltip content={resource.name} className='application-details__item_text resources-table__tooltip-anchor' />
                    </CellLink>
                </div>
                <div className='columns small-1 large-2 resources-table__col-cluster'>
                    <CellLink href={rowLink.href} onClick={rowLink.onClick}>
                        <TruncatedTextTooltip content={resource.clusterName || resource.clusterServer || ''} className='resources-table__cell-text resources-table__tooltip-anchor'>
                            <Cluster server={resource.clusterServer} name={resource.clusterName} />
                        </TruncatedTextTooltip>
                    </CellLink>
                </div>
                <div className='columns small-2 resources-table__col-application'>
                    <Tooltip content={resource.appName} enabled={!!resource.appName && appLinkIsTruncated}>
                        <a ref={appLinkRef} className='resources-table__application-link' href={appLink.href} onClick={appLink.onClick}>
                            {resource.appName}
                        </a>
                    </Tooltip>
                </div>
                <div className='columns small-3 large-2 resources-table__status-col'>
                    <div className='resources-table__status-line'>
                        <AppUtils.HealthStatusIcon state={resourceHealthState(resource)} /> <span>{resourceHealthStatus(resource)}</span>
                    </div>
                    {resource.status && (
                        <div className='resources-table__status-line'>
                            <AppUtils.ComparisonStatusIcon status={resource.status} resource={resource} label={true} />
                        </div>
                    )}
                </div>
            </div>
            <div className='application-details__node-menu resources-table__row-menu' onClick={e => e.stopPropagation()}>
                <ActionMenu
                    items={[
                        {title: 'Resource Details', iconClassName: 'fa fa-fw fa-info-circle', action: () => openResourceDetailsInApplication(ctx, resource)},
                        {title: 'Open Application', iconClassName: 'fa fa-fw fa-external-link-alt', action: () => navigateToApplication(ctx, resource)}
                    ]}
                />
            </div>
        </div>
    );
};
