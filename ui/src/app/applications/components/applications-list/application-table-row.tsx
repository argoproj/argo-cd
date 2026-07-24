import {DropDownMenu, NotificationType, Tooltip} from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
import {Cluster} from '../../../shared/components';
import {AuthSettingsCtx, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {NoticeIcon} from '../application-notice/notice-icon';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {getAppDefaultSource, OperationState, getApplicationLinkURL, getManagedByURL, MANAGED_BY_URL_INVALID_TEXT, MANAGED_BY_URL_INVALID_TOOLTIP} from '../utils';
import {isValidManagedByURL} from '../../../shared/utils';
import {ApplicationsLabels} from './applications-labels';
import {ApplicationsSource} from './applications-source';
import {CellLink} from './cell-link';
import {EntryField, EntryFieldList} from './entry-fields';
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';

import './entry-fields.scss';

export interface ApplicationTableRowProps {
    app: models.Application;
    selected: boolean;
    pref: ViewPreferences;
    ctx: ContextApis;
    syncApplication: (appName: string, appNamespace: string) => void;
    refreshApplication: (appName: string, appNamespace: string) => void;
    deleteApplication: (appName: string, appNamespace: string) => void;
}

export const ApplicationTableRow = ({app, selected, pref, ctx, syncApplication, refreshApplication, deleteApplication}: ApplicationTableRowProps) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const favList = pref.appList.favoritesAppList || [];
    const healthStatus = app.status.health.status;
    const linkInfo = getApplicationLinkURL(app, ctx.baseHref);
    const source = getAppDefaultSource(app);
    const managedByURL = getManagedByURL(app);
    const managedByURLInvalid = !!managedByURL && !isValidManagedByURL(managedByURL);

    const view = pref.appDetails.view;
    const appLink = AppUtils.getAppListLink(ctx, app, view);
    const qualifiedName = AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled);

    const handleFavoriteToggle = (e: React.MouseEvent) => {
        e.stopPropagation();
        if (favList?.includes(app.metadata.name)) {
            favList.splice(favList.indexOf(app.metadata.name), 1);
        } else {
            favList.push(app.metadata.name);
        }
        services.viewPreferences.updatePreferences({appList: {...pref.appList, favoritesAppList: favList}});
    };

    const handleExternalLinkClick = (e: React.MouseEvent) => {
        e.stopPropagation();
        if (managedByURLInvalid) {
            ctx.notifications.show({
                content: (
                    <div>
                        <div style={{fontWeight: 600}}>{MANAGED_BY_URL_INVALID_TEXT}</div>
                        <div style={{marginTop: 6}}>{MANAGED_BY_URL_INVALID_TOOLTIP}</div>
                    </div>
                ),
                type: NotificationType.Warning
            });
            return;
        }
        if (linkInfo.isExternal) {
            window.open(linkInfo.url, '_blank', 'noopener,noreferrer');
        } else {
            ctx.navigation.goto(appLink.path, {view});
        }
    };

    return (
        // role=group + aria-label on the row card ties its columns/<dl>s together as one labelled
        // record, recovering the "these fields all describe one application" relationship that
        // splitting into per-column <dl>s would otherwise lose. (Matches the tile's outer div.)
        <div
            role='group'
            aria-label={qualifiedName}
            className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <div className='row applications-list__table-row'>
                {/* The overlay anchor is the row's accessible link: a real link in tab order with an
                    aria-label so screen readers announce the application name once per row. It sits
                    behind the row's interactive children (lifted via z-index in the SCSS) so the
                    visible buttons, dropdowns, status icons, etc. still receive their own clicks. */}
                <a className='applications-list__table-row__overlay-link' href={appLink.href} onClick={appLink.onClick} aria-label={qualifiedName} />

                {/* Favourite + external URLs: its own content-width `shrink` column. The Project/Name
                    <dl> that follows expands to fill the rest, so together they occupy the old
                    small-4 region — no wrapping meta-column/small-4 div needed. */}
                <div className='applications-list__fav-col columns shrink'>
                    <Tooltip content={favList?.includes(app.metadata.name) ? 'Remove Favorite' : 'Add Favorite'}>
                        <button type='button' onClick={handleFavoriteToggle}>
                            <i
                                className={favList?.includes(app.metadata.name) ? 'fas fa-star' : 'far fa-star'}
                                style={{
                                    cursor: 'pointer',
                                    color: favList?.includes(app.metadata.name) ? '#FFCE25' : '#8fa4b1'
                                }}
                            />
                        </button>
                    </Tooltip>
                    <ApplicationURLs urls={app.status.summary?.externalURLs} />
                </div>
                {/* One flat <dl> for the whole record, laid out as the legacy three visual columns —
                    Project/Name · Source/Destination · Status, each a (label | value) pair stacked
                    two-high (see `.applications-list__meta-flat` in entry-fields.scss, which pins each
                    field to its grid cell). Replaces the former three per-column <dl>s; the fav-col
                    and actions menu stay outside it. Field order below feeds the explicit placement. */}
                <EntryFieldList variant='table' className='applications-list__meta-flat'>
                    <EntryField name='project' label='Project'>
                        {app.spec.project}
                    </EntryField>
                    <EntryField name='name' label='Name'>
                        {/* NoticeIcon before the name so it stays visible when the name truncates. */}
                        <NoticeIcon annotations={app.metadata.annotations} />
                        <Tooltip
                            content={
                                <>
                                    {app.metadata.name}
                                    <br />
                                    <Moment fromNow={true} ago={true}>
                                        {app.metadata.creationTimestamp}
                                    </Moment>
                                </>
                            }>
                            <a className='applications-list__table-row-name' href={appLink.href} onClick={appLink.onClick} tabIndex={-1}>
                                {app.metadata.name}
                            </a>
                        </Tooltip>
                        <button
                            type='button'
                            className={managedByURLInvalid ? 'managed-by-url-invalid' : undefined}
                            onClick={handleExternalLinkClick}
                            style={{marginLeft: '0.5em', cursor: managedByURLInvalid ? 'not-allowed' : undefined}}
                            title={managedByURLInvalid ? MANAGED_BY_URL_INVALID_TEXT : `Link: ${linkInfo.url}\nmanaged-by-url: ${managedByURL || 'none'}`}>
                            <i className='fa fa-window-maximize' />
                        </button>
                    </EntryField>
                    <EntryField name='source' label='Source' valueClassName='applications-table-source'>
                        <CellLink href={appLink.href} onClick={appLink.onClick} className='applications-table-source__link'>
                            <ApplicationsSource source={source} />
                        </CellLink>
                        <CellLink href={appLink.href} onClick={appLink.onClick} className='applications-table-source__labels'>
                            <ApplicationsLabels app={app} />
                        </CellLink>
                    </EntryField>
                    <EntryField name='destination' label='Destination'>
                        <CellLink href={appLink.href} onClick={appLink.onClick}>
                            <Cluster server={app.spec.destination.server} name={app.spec.destination.name} />/{app.spec.destination.namespace}
                        </CellLink>
                    </EntryField>
                    <EntryField name='status' label='Status'>
                        <CellLink href={appLink.href} onClick={appLink.onClick}>
                            <AppUtils.HealthStatusIcon state={app.status.health} /> <span>{app.status.health.status}</span> <br />
                            {app.status.sourceHydrator?.currentOperation && (
                                <>
                                    <AppUtils.HydrateOperationPhaseIcon operationState={app.status.sourceHydrator.currentOperation} />{' '}
                                    <span>{app.status.sourceHydrator.currentOperation.phase}</span> <br />
                                </>
                            )}
                            <AppUtils.ComparisonStatusIcon status={app.status.sync.status} />
                            <span>{app.status.sync.status}</span> <OperationState app={app} quiet={true} />
                        </CellLink>
                    </EntryField>
                </EntryFieldList>

                <DropDownMenu
                    anchor={() => (
                        <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                            <i className='fa fa-ellipsis-v' />
                        </button>
                    )}
                    items={[
                        {
                            title: 'Sync',
                            iconClassName: 'fa fa-fw fa-sync',
                            action: () => syncApplication(app.metadata.name, app.metadata.namespace)
                        },
                        {
                            title: 'Refresh',
                            iconClassName: 'fa fa-fw fa-redo',
                            action: () => refreshApplication(app.metadata.name, app.metadata.namespace)
                        },
                        {
                            title: 'Delete',
                            iconClassName: 'fa fa-fw fa-times-circle',
                            action: () => deleteApplication(app.metadata.name, app.metadata.namespace)
                        }
                    ]}
                />
            </div>
        </div>
    );
};
