import {NotificationType, Tooltip} from 'argo-ui';
import * as React from 'react';
import {ContextApis, AuthSettingsCtx} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {getApplicationLinkURL, getManagedByURL, getAppSetHealthStatus, MANAGED_BY_URL_INVALID_TEXT, MANAGED_BY_URL_INVALID_TOOLTIP} from '../utils';
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';
import {isValidManagedByURL} from '../../../shared/utils';

export interface AppSetTileProps {
    appSet: models.ApplicationSet;
    selected: boolean;
    pref: ViewPreferences;
    ctx: ContextApis;
    tileRef?: React.RefObject<HTMLDivElement>;
}

export const AppSetTile = ({appSet, selected, pref, ctx, tileRef}: AppSetTileProps) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const favList = pref.appList.favoritesAppList || [];

    const linkInfo = getApplicationLinkURL(appSet, ctx.baseHref);
    const healthStatus = getAppSetHealthStatus(appSet);
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
        <div
            ref={tileRef}
            className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <a
                className='row applications-tiles__wrapper'
                href={appSetLink.href}
                onClick={appSetLink.onClick}
                draggable={false}
                aria-label={AppUtils.appQualifiedName(appSet, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}>
                <div className={`columns small-12 applications-list__info qe-applications-list-${AppUtils.appInstanceName(appSet)} applications-tiles__item`}>
                    {/* Header row with icon, title, and action buttons */}
                    <div className='row'>
                        <div className='columns small-11 applications-tiles__title-col'>
                            <i className='icon argo-icon-applicationset' />
                            <Tooltip content={AppUtils.appInstanceName(appSet)}>
                                <span className='applications-list__title'>{AppUtils.appQualifiedName(appSet, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}</span>
                            </Tooltip>
                        </div>
                        {/* Empty placeholder — the actual buttons live outside the anchor as an
                            absolutely-positioned sibling, so we don't nest interactive content in <a>. */}
                        <div className='columns small-1' aria-hidden='true' />
                    </div>

                    <div className='applications-tiles__fields'>
                        {/* Labels row */}
                        <div className='row applications-tiles__field-row'>
                            <div className='columns applications-tiles__field-label' title='Labels:'>
                                Labels:
                            </div>
                            <div className='columns applications-tiles__field-value'>
                                <Tooltip
                                    zIndex={4}
                                    content={
                                        <div>
                                            {Object.keys(appSet.metadata.labels || {})
                                                .map(label => ({label, value: appSet.metadata.labels[label]}))
                                                .map(item => (
                                                    <div key={item.label}>
                                                        {item.label}={item.value}
                                                    </div>
                                                ))}
                                        </div>
                                    }>
                                    <span>
                                        {Object.keys(appSet.metadata.labels || {})
                                            .map(label => `${label}=${appSet.metadata.labels[label]}`)
                                            .join(', ')}
                                    </span>
                                </Tooltip>
                            </div>
                        </div>

                        {/* Status row */}
                        <div className='row applications-tiles__field-row'>
                            <div className='columns applications-tiles__field-label' title='Status:'>
                                Status:
                            </div>
                            <div className='columns applications-tiles__field-value' qe-id='applications-tiles-health-status'>
                                <AppUtils.HealthStatusIcon state={{status: healthStatus, message: ''}} /> {healthStatus}
                            </div>
                        </div>

                        {/* Applications count row */}
                        <div className='row applications-tiles__field-row'>
                            <div className='columns applications-tiles__field-label' title='Applications:'>
                                Applications:
                            </div>
                            <div className='columns applications-tiles__field-value'>{appSet.status?.resourcesCount ?? appSet.status?.resources?.length ?? 0}</div>
                        </div>

                        {/* Created At row */}
                        <div className='row applications-tiles__field-row'>
                            <div className='columns applications-tiles__field-label' title='Age:'>
                                Created At:
                            </div>
                            <div className='columns applications-tiles__field-value'>{AppUtils.formatCreationTimestamp(appSet.metadata.creationTimestamp)}</div>
                        </div>
                    </div>
                </div>
            </a>

            {/* Header buttons — sibling of the anchor (not nested) so the markup stays valid. */}
            <div className='applications-tiles__header-buttons applications-list__external-link'>
                {managedByURLInvalid ? (
                    <button type='button' className='managed-by-url-invalid' onClick={handleExternalLinkClick} style={{cursor: 'not-allowed'}} title={MANAGED_BY_URL_INVALID_TEXT}>
                        <i className='fa fa-window-maximize' />
                    </button>
                ) : (
                    <button type='button' onClick={handleExternalLinkClick} title={managedByURL ? `Managed by: ${managedByURL}` : 'Open application'}>
                        <i className='fa fa-window-maximize' />
                    </button>
                )}
                <button title={favList?.includes(appSet.metadata.name) ? 'Remove Favorite' : 'Add Favorite'} className='large-text-height' onClick={handleFavoriteToggle}>
                    <i
                        className={favList?.includes(appSet.metadata.name) ? 'fas fa-star fa-lg' : 'far fa-star fa-lg'}
                        style={{
                            cursor: 'pointer',
                            margin: '-1px 0px 0px 7px',
                            color: favList?.includes(appSet.metadata.name) ? '#FFCE25' : '#8fa4b1'
                        }}
                    />
                </button>
            </div>
        </div>
    );
};
