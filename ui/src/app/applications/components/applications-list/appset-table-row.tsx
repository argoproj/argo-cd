import {NotificationType, Tooltip} from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
import {ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {getApplicationLinkURL, getManagedByURL, getAppSetHealthStatus, MANAGED_BY_URL_INVALID_TEXT, MANAGED_BY_URL_INVALID_TOOLTIP} from '../utils';
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';
import {isValidManagedByURL} from '../../../shared/utils';

export interface AppSetTableRowProps {
    appSet: models.ApplicationSet;
    selected: boolean;
    pref: ViewPreferences;
    ctx: ContextApis;
}

export const AppSetTableRow = ({appSet, selected, pref, ctx}: AppSetTableRowProps) => {
    const favList = pref.appList.favoritesAppList || [];
    const healthStatus = getAppSetHealthStatus(appSet);
    const linkInfo = getApplicationLinkURL(appSet, ctx.baseHref);
    const managedByURL = getManagedByURL(appSet);
    const managedByURLInvalid = !!managedByURL && !isValidManagedByURL(managedByURL);

    const appSetPath = `/${AppUtils.getAppUrl(appSet)}`;
    const appSetHref = `${ctx.baseHref}${AppUtils.getAppUrl(appSet)}`;

    const handleRowClick = (e: React.MouseEvent<HTMLAnchorElement>) => {
        if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) {
            return;
        }
        e.preventDefault();
        ctx.navigation.goto(appSetPath, {}, {event: e});
    };

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
            ctx.navigation.goto(`/${AppUtils.getAppUrl(appSet)}`);
        }
    };

    return (
        <div className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <div className='row applications-list__table-row'>
                {/* The name anchor below is the row's sole accessible link; this overlay is
                    only for pointer clicks on non-name areas, so hide it from the a11y tree
                    and tab order to avoid a duplicate tab stop / announcement. */}
                <a className='applications-list__table-row__overlay-link' href={appSetHref} onClick={handleRowClick} tabIndex={-1} aria-hidden='true' />
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
                                <a className='applications-list__table-row-name' href={appSetHref} onClick={handleRowClick}>
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

                {/* Status column (takes remaining space since no Source/Destination for AppSets) */}
                <div className='columns small-8'>
                    <AppUtils.HealthStatusIcon state={{status: healthStatus, message: ''}} /> <span>{healthStatus}</span>
                </div>
            </div>
        </div>
    );
};
