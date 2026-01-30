import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {ContextApis, AuthSettingsCtx} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {getApplicationLinkURL, getManagedByURL, getAppSetHealthStatus} from '../utils';
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';

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
        if (linkInfo.isExternal) {
            window.open(linkInfo.url, '_blank', 'noopener,noreferrer');
        } else {
            ctx.navigation.goto(`/${AppUtils.getAppUrl(appSet)}`);
        }
    };

    return (
        <div
            ref={tileRef}
            className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <div className='row applications-tiles__wrapper' onClick={e => ctx.navigation.goto(`/${AppUtils.getAppUrl(appSet)}`, {view: pref.appDetails.view}, {event: e})}>
                <div className={`columns small-12 applications-list__info qe-applications-list-${AppUtils.appInstanceName(appSet)} applications-tiles__item`}>
                    {/* Header row with icon, title, and action buttons */}
                    <div className='row'>
                        <div className='columns small-11'>
                            <i className='icon argo-icon-git' />
                            <Tooltip content={AppUtils.appInstanceName(appSet)}>
                                <span className='applications-list__title'>{AppUtils.appQualifiedName(appSet, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}</span>
                            </Tooltip>
                        </div>
                        <div className='columns small-1'>
                            <div className='applications-list__external-link'>
                                <button onClick={handleExternalLinkClick} title={getManagedByURL(appSet) ? `Managed by: ${getManagedByURL(appSet)}` : 'Open application'}>
                                    <i className='fa fa-external-link-alt' />
                                </button>
                                <button
                                    title={favList?.includes(appSet.metadata.name) ? 'Remove Favorite' : 'Add Favorite'}
                                    className='large-text-height'
                                    onClick={handleFavoriteToggle}>
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
                    </div>

                    {/* Labels row */}
                    <div className='row'>
                        <div className='columns small-3' title='Labels:'>
                            Labels:
                        </div>
                        <div className='columns small-9'>
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
                    <div className='row'>
                        <div className='columns small-3' title='Status:'>
                            Status:
                        </div>
                        <div className='columns small-9' qe-id='applications-tiles-health-status'>
                            <AppUtils.HealthStatusIcon state={{status: healthStatus, message: ''}} /> {healthStatus}
                        </div>
                    </div>

                    {/* Applications count row */}
                    <div className='row'>
                        <div className='columns small-3' title='Applications:'>
                            Applications:
                        </div>
                        <div className='columns small-9'>{appSet.status?.resourcesCount ?? appSet.status?.resources?.length ?? 0}</div>
                    </div>

                    {/* Created At row */}
                    <div className='row'>
                        <div className='columns small-3' title='Age:'>
                            Created At:
                        </div>
                        <div className='columns small-9'>{AppUtils.formatCreationTimestamp(appSet.metadata.creationTimestamp)}</div>
                    </div>
                </div>
            </div>
        </div>
    );
};
