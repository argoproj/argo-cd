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
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';

export interface ApplicationTableRowProps {
    app: models.Application;
    selected: boolean;
    pref: ViewPreferences;
    ctx: ContextApis;
    syncApplication: (appName: string, appNamespace: string) => void;
    refreshApplication: (appName: string, appNamespace: string) => void;
    deleteApplication: (appName: string, appNamespace: string) => void;
}

// Wraps a cell's content in an <a> so middle-click / right-click / status-bar URL preview
// work on the cell itself. tabIndex={-1} keeps it out of the keyboard tab order — the
// row's overlay anchor is the single tab stop carrying the row's link semantics. The
// cell content remains in the a11y tree so screen readers can still read Source / Labels
// / Destination; the trade-off is that SR link lists will show one entry per CellLink
// (same href as the overlay), which is the accepted cost for preserving mouse affordances
// on cell content. Defined at module scope so children like <Cluster> don't remount on
// each parent re-render.
const CellLink = ({
    href,
    onClick,
    className,
    children
}: {
    href: string;
    onClick: (e: React.MouseEvent<HTMLAnchorElement>) => void;
    className?: string;
    children: React.ReactNode;
}) => (
    <a className={`applications-list__table-row__cell-link${className ? ` ${className}` : ''}`} href={href} onClick={onClick} tabIndex={-1}>
        {children}
    </a>
);

export const ApplicationTableRow = ({app, selected, pref, ctx, syncApplication, refreshApplication, deleteApplication}: ApplicationTableRowProps) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const favList = pref.appList.favoritesAppList || [];
    const healthStatus = app.status.health.status;
    const linkInfo = getApplicationLinkURL(app, ctx.baseHref);
    const source = getAppDefaultSource(app);
    const managedByURL = getManagedByURL(app);
    const managedByURLInvalid = !!managedByURL && !isValidManagedByURL(managedByURL);

    const appPath = `/${AppUtils.getAppUrl(app)}`;
    const view = pref.appDetails.view;
    const appHref = `${ctx.baseHref}${AppUtils.getAppUrl(app)}${view ? `?view=${encodeURIComponent(view)}` : ''}`;

    const handleRowClick = (e: React.MouseEvent<HTMLAnchorElement>) => {
        if (e.metaKey || e.ctrlKey || e.shiftKey) {
            return;
        }
        e.preventDefault();
        ctx.navigation.goto(appPath, {view}, {event: e});
    };

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
            ctx.navigation.goto(appPath, {view});
        }
    };

    return (
        <div className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <div className={`row applications-list__table-row ${app.status.sourceHydrator?.currentOperation ? 'applications-table-row--with-hydrator' : ''}`}>
                {/* The overlay anchor is the row's accessible link: a real link in tab order with an
                    aria-label so screen readers announce the application name once per row. It sits
                    behind the row's interactive children (lifted via z-index in the SCSS) so the
                    visible buttons, dropdowns, status icons, etc. still receive their own clicks. */}
                <a
                    className='applications-list__table-row__overlay-link'
                    href={appHref}
                    onClick={handleRowClick}
                    aria-label={AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                />
                {/* First column: Favorite, URLs, Project, Name */}
                <div className='columns small-4'>
                    <div className='row'>
                        <div className='columns small-2'>
                            <div>
                                <Tooltip content={favList?.includes(app.metadata.name) ? 'Remove Favorite' : 'Add Favorite'}>
                                    <button onClick={handleFavoriteToggle}>
                                        <i
                                            className={favList?.includes(app.metadata.name) ? 'fas fa-star' : 'far fa-star'}
                                            style={{
                                                cursor: 'pointer',
                                                marginRight: '7px',
                                                color: favList?.includes(app.metadata.name) ? '#FFCE25' : '#8fa4b1'
                                            }}
                                        />
                                    </button>
                                </Tooltip>
                                <ApplicationURLs urls={app.status.summary?.externalURLs} />
                            </div>
                        </div>
                        <div className='show-for-xxlarge columns small-4'>Project:</div>
                        <div className='columns small-12 xxlarge-6'>{app.spec.project}</div>
                    </div>
                    <div className='row'>
                        <div className='columns small-2' />
                        <div className='show-for-xxlarge columns small-4'>Name:</div>
                        <div className='columns small-12 xxlarge-6'>
                            {/* Rendered before the name so it stays visible when the name truncates with ellipsis;
                                the column's `overflow:hidden; white-space:nowrap` (argo-ui table-list) clips trailing
                                inline children. The tile view does the opposite because there the title wraps. */}
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
                                <a className='applications-list__table-row-name' href={appHref} onClick={handleRowClick}>
                                    {app.metadata.name}
                                </a>
                            </Tooltip>
                            <button
                                type='button'
                                className={managedByURLInvalid ? 'managed-by-url-invalid' : undefined}
                                onClick={handleExternalLinkClick}
                                style={{marginLeft: '0.5em', cursor: managedByURLInvalid ? 'not-allowed' : undefined}}
                                title={managedByURLInvalid ? MANAGED_BY_URL_INVALID_TEXT : `Link: ${linkInfo.url}\nmanaged-by-url: ${managedByURL || 'none'}`}>
                                <i className='fa fa-external-link-alt' />
                            </button>
                        </div>
                    </div>
                </div>

                {/* Second column: Source and Destination — wrapped in CellLink so each cell
                    behaves as a real link (middle-click / right-click / status-bar URL preview).
                    Keyboard users tab to the overlay anchor instead (CellLink uses tabIndex=-1). */}
                <div className='columns small-6'>
                    <div className='row'>
                        <div className='show-for-xxlarge columns small-2'>Source:</div>
                        <div className='columns small-12 xxlarge-10 applications-table-source' style={{position: 'relative'}}>
                            <CellLink href={appHref} onClick={handleRowClick} className='applications-table-source__link'>
                                <ApplicationsSource source={source} />
                            </CellLink>
                            <CellLink href={appHref} onClick={handleRowClick} className='applications-table-source__labels'>
                                <ApplicationsLabels app={app} />
                            </CellLink>
                        </div>
                    </div>
                    <div className='row'>
                        <div className='show-for-xxlarge columns small-2'>Destination:</div>
                        <div className='columns small-12 xxlarge-10'>
                            <CellLink href={appHref} onClick={handleRowClick}>
                                <Cluster server={app.spec.destination.server} name={app.spec.destination.name} />/{app.spec.destination.namespace}
                            </CellLink>
                        </div>
                    </div>
                </div>

                {/* Third column: Status and Actions */}
                <div className='columns small-2'>
                    <AppUtils.HealthStatusIcon state={app.status.health} /> <span>{app.status.health.status}</span> <br />
                    {app.status.sourceHydrator?.currentOperation && (
                        <>
                            <AppUtils.HydrateOperationPhaseIcon operationState={app.status.sourceHydrator.currentOperation} />{' '}
                            <span>{app.status.sourceHydrator.currentOperation.phase}</span> <br />
                        </>
                    )}
                    <AppUtils.ComparisonStatusIcon status={app.status.sync.status} />
                    <span>{app.status.sync.status}</span> <OperationState app={app} quiet={true} />
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
        </div>
    );
};
