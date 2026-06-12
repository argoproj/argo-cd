import {NotificationType, Tooltip} from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
import {AuthSettingsCtx, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {getApplicationLinkURL, getManagedByURL, getAppSetHealthStatus, MANAGED_BY_URL_INVALID_TEXT, MANAGED_BY_URL_INVALID_TOOLTIP} from '../utils';
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';
import {isValidManagedByURL} from '../../../shared/utils';
import {CellLink} from './cell-link';

export interface AppSetTableRowProps {
    appSet: models.ApplicationSet;
    selected: boolean;
    pref: ViewPreferences;
    ctx: ContextApis;
}

export const AppSetTableRow = ({appSet, selected, pref, ctx}: AppSetTableRowProps) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const favList = pref.appList.favoritesAppList || [];
    const healthStatus = getAppSetHealthStatus(appSet);
    const linkInfo = getApplicationLinkURL(appSet, ctx.baseHref);
    const managedByURL = getManagedByURL(appSet);
    const managedByURLInvalid = !!managedByURL && !isValidManagedByURL(managedByURL);

    // AppSet pages don't support the Application details `view` param, so the link is view-less.
    const appSetLink = AppUtils.getAppListLink(ctx, appSet);

    const handleFavoriteToggle = (e: React.MouseEvent) => {
        e.stopPropagation();
        if (favList?.includes(appSet.metadata.name)) {
            favList.splice(favList.indexOf(appSet.metadata.name), 1);
        } else {
            favList.push(appSet.metadata.name);
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
            ctx.navigation.goto(appSetLink.path);
        }
    };

    return (
        <div className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <div className='row applications-list__table-row'>
                {/* The overlay anchor is the row's accessible link: a real link in tab order with an
                    aria-label so screen readers announce the appset name once per row. It sits
                    behind the row's interactive children (lifted via z-index in the SCSS) so the
                    visible buttons and dropdowns still receive their own clicks. */}
                <a
                    className='applications-list__table-row__overlay-link'
                    href={appSetLink.href}
                    onClick={appSetLink.onClick}
                    aria-label={AppUtils.appQualifiedName(appSet, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                />
                {/* First column: Favorite, Kind, Name */}
                <div className='columns small-4'>
                    <div className='row'>
                        <div className='columns small-2'>
                            <div>
                                <Tooltip content={favList?.includes(appSet.metadata.name) ? 'Remove Favorite' : 'Add Favorite'}>
                                    <button onClick={handleFavoriteToggle}>
                                        <i
                                            className={favList?.includes(appSet.metadata.name) ? 'fas fa-star' : 'far fa-star'}
                                            style={{
                                                cursor: 'pointer',
                                                marginRight: '7px',
                                                color: favList?.includes(appSet.metadata.name) ? '#FFCE25' : '#8fa4b1'
                                            }}
                                        />
                                    </button>
                                </Tooltip>
                            </div>
                        </div>
                        <div className='show-for-xxlarge columns small-4'>Kind:</div>
                        <div className='columns small-12 xxlarge-6'>ApplicationSet</div>
                    </div>
                    <div className='row'>
                        <div className='columns small-2' />
                        <div className='show-for-xxlarge columns small-4'>Name:</div>
                        <div className='columns small-12 xxlarge-6'>
                            <Tooltip
                                content={
                                    <>
                                        {appSet.metadata.name}
                                        <br />
                                        <Moment fromNow={true} ago={true}>
                                            {appSet.metadata.creationTimestamp}
                                        </Moment>
                                    </>
                                }>
                                <a className='applications-list__table-row-name' href={appSetLink.href} onClick={appSetLink.onClick} tabIndex={-1}>
                                    {appSet.metadata.name}
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

                {/* Status column (takes remaining space since no Source/Destination for AppSets) —
                    wrapped in CellLink so the status icon (which carries a `title` and is lifted
                    above the overlay) navigates on click instead of being a dead zone. */}
                <div className='columns small-8'>
                    <CellLink href={appSetLink.href} onClick={appSetLink.onClick}>
                        <AppUtils.HealthStatusIcon state={{status: healthStatus, message: ''}} /> <span>{healthStatus}</span>
                    </CellLink>
                </div>
            </div>
        </div>
    );
};
