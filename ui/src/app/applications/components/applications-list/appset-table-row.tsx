import {Tooltip} from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
import {AuthSettingsCtx, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {getAppSetHealthStatus} from '../utils';
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';
import {CellLink} from '../../../shared/components';
import {EntryField, EntryFieldList} from './entry-fields';

import './entry-fields.scss';

export interface AppSetTableRowProps {
    appSet: models.ApplicationSet;
    selected: boolean;
    pref: ViewPreferences;
    ctx: ContextApis;
}

export const AppSetTableRow = ({appSet, selected, pref, ctx}: AppSetTableRowProps) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const appSetDisplayName = AppUtils.appQualifiedName(appSet, useAuthSettingsCtx?.appsInAnyNamespaceEnabled);
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
        // role=group + aria-label ties the row's columns/<dl>s together as one labelled record.
        <div
            role='group'
            aria-label={appSetDisplayName}
            className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <div className='row applications-list__table-row'>
                {/* The overlay anchor is the row's accessible link: a real link in tab order with an
                    aria-label so screen readers announce the appset name once per row. It sits
                    behind the row's interactive children (lifted via z-index in the SCSS) so the
                    visible buttons and dropdowns still receive their own clicks. */}
                <a className='applications-list__table-row__overlay-link' href={appSetLink.href} onClick={appSetLink.onClick} aria-label={appSetDisplayName} />
                {/* Favourite: its own content-width `shrink` column. The Kind/Name <dl> that follows
                    expands to fill the rest, so together they occupy the old small-4 region. */}
                <div className='applications-list__fav-col columns shrink'>
                    <Tooltip content={isFav ? 'Remove Favorite' : 'Add Favorite'}>
                        <button type='button' onClick={handleFavoriteToggle}>
                            <i
                                className={isFav ? 'fas fa-star' : 'far fa-star'}
                                style={{
                                    cursor: 'pointer',
                                    color: isFav ? '#FFCE25' : '#8fa4b1'
                                }}
                            />
                        </button>
                    </Tooltip>
                </div>
                <EntryFieldList variant='table' className='applications-list__meta-rows'>
                    <EntryField name='kind' label='Kind'>
                        ApplicationSet
                    </EntryField>
                    <EntryField name='name' label='Name' valueClassName='applications-list__name-value'>
                        <Tooltip
                            content={
                                <>
                                    {appSetDisplayName}
                                    <br />
                                    <Moment fromNow={true} ago={true}>
                                        {appSet.metadata.creationTimestamp}
                                    </Moment>
                                </>
                            }>
                            <a className='applications-list__table-row-name' href={appSetLink.href} onClick={appSetLink.onClick} tabIndex={-1}>
                                {appSetDisplayName}
                            </a>
                        </Tooltip>
                    </EntryField>
                </EntryFieldList>

                {/* Status: a single-pair <dl> filling the remaining space (no Source/Destination for
                    AppSets). Its term ("Status") is sr-only at all widths; the value is wrapped in a
                    CellLink so the status icon (lifted above the overlay) navigates on click. */}
                <EntryFieldList variant='table' className='columns small-8'>
                    <EntryField name='status' label='Status'>
                        <CellLink href={appSetLink.href} onClick={appSetLink.onClick}>
                            <AppUtils.HealthStatusIcon state={{status: healthStatus, message: ''}} /> <span>{healthStatus}</span>
                        </CellLink>
                    </EntryField>
                </EntryFieldList>
            </div>
        </div>
    );
};
