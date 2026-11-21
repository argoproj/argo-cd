import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {ContextApis, AuthSettingsCtx} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {getAppSetHealthStatus} from '../utils';
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';
import {EntryField, EntryFieldList} from './entry-fields';

import './entry-fields.scss';

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
    const isFav = AppUtils.isFavorite(favList, appSet);

    const healthStatus = getAppSetHealthStatus(appSet);

    // AppSet pages don't support the Application details `view` param, so the link is view-less.
    const appSetLink = AppUtils.getAppListLink(ctx, appSet);

    const handleFavoriteToggle = (e: React.MouseEvent) => {
        e.stopPropagation();
        services.viewPreferences.updatePreferences({appList: {...pref.appList, favoritesAppList: AppUtils.toggleFavorite(favList, appSet)}});
    };

    return (
        // role=group + aria-label names the card as one labelled record, matching the app tile.
        <div
            ref={tileRef}
            role='group'
            aria-label={AppUtils.appQualifiedName(appSet, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
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

                    <EntryFieldList variant='tile'>
                        <EntryField name='labels' label='Labels'>
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
                        </EntryField>
                        <EntryField name='status' label='Status'>
                            <span qe-id='applications-tiles-health-status'>
                                <AppUtils.HealthStatusIcon state={{status: healthStatus, message: ''}} /> {healthStatus}
                            </span>
                        </EntryField>
                        <EntryField name='applications' label='Applications'>
                            {appSet.status?.resourcesCount ?? appSet.status?.resources?.length ?? 0}
                        </EntryField>
                        <EntryField name='createdAt' label='Created At'>
                            {AppUtils.formatCreationTimestamp(appSet.metadata.creationTimestamp)}
                        </EntryField>
                    </EntryFieldList>
                </div>
            </a>

            {/* Header buttons — sibling of the anchor (not nested) so the markup stays valid. */}
            <div className='applications-tiles__header-buttons applications-list__external-link'>
                <button title={isFav ? 'Remove Favorite' : 'Add Favorite'} className='large-text-height' onClick={handleFavoriteToggle}>
                    <i
                        className={isFav ? 'fas fa-star fa-lg' : 'far fa-star fa-lg'}
                        style={{
                            cursor: 'pointer',
                            margin: '-1px 0px 0px 0px',
                            color: isFav ? '#FFCE25' : '#8fa4b1'
                        }}
                    />
                </button>
            </div>
        </div>
    );
};
