import {NotificationType, Tooltip} from 'argo-ui';
import classNames from 'classnames';
import * as React from 'react';
import {Cluster} from '../../../shared/components';
import {ContextApis, AuthSettingsCtx} from '../../../shared/context';
import * as models from '../../../shared/models';
import {NoticeIcon} from '../application-notice/notice-icon';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {getAppDefaultSource, OperationState, getApplicationLinkURL, getManagedByURL, MANAGED_BY_URL_INVALID_TEXT, MANAGED_BY_URL_INVALID_TOOLTIP} from '../utils';
import {isValidManagedByURL} from '../../../shared/utils';
import {EntryField, EntryFieldList} from './entry-fields';
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';

import './entry-fields.scss';

export interface ApplicationTileProps {
    app: models.Application;
    selected: boolean;
    pref: ViewPreferences;
    ctx: ContextApis;
    tileRef?: React.RefObject<HTMLDivElement>;
    syncApplication: (appName: string, appNamespace: string) => void;
    refreshApplication: (appName: string, appNamespace: string) => void;
    deleteApplication: (appName: string, appNamespace: string) => void;
}

export const ApplicationTile = ({app, selected, pref, ctx, tileRef, syncApplication, refreshApplication, deleteApplication}: ApplicationTileProps) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const favList = pref.appList.favoritesAppList || [];

    const source = getAppDefaultSource(app);
    const isOci = source?.repoURL?.startsWith('oci://');
    const targetRevision = source ? source.targetRevision || 'HEAD' : 'Unknown';
    const linkInfo = getApplicationLinkURL(app, ctx.baseHref);
    const healthStatus = app.status.health.status;
    const managedByURL = getManagedByURL(app);
    const managedByURLInvalid = !!managedByURL && !isValidManagedByURL(managedByURL);

    const view = pref.appDetails.view;
    const appLink = AppUtils.getAppListLink(ctx, app, view);

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
        // role=group + aria-label names the card as one labelled record, matching the table row.
        <div
            ref={tileRef}
            role='group'
            aria-label={AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
            className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <a
                className='row applications-tiles__wrapper'
                href={appLink.href}
                onClick={appLink.onClick}
                draggable={false}
                aria-label={AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}>
                <div className={`columns small-12 applications-list__info qe-applications-list-${AppUtils.appInstanceName(app)} applications-tiles__item`}>
                    {/* Header row with icon, title, and action buttons */}
                    <div className='row'>
                        <div className={(app.status.summary?.externalURLs?.length > 0 ? 'columns small-10' : 'columns small-11') + ' applications-tiles__title-col'}>
                            <i className={'icon argo-icon-' + (source?.chart != null ? 'helm' : isOci ? 'oci applications-tiles__item__small' : 'git')} />
                            <Tooltip content={AppUtils.appInstanceName(app)}>
                                <span className='applications-list__title'>{AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}</span>
                            </Tooltip>
                            <NoticeIcon annotations={app.metadata.annotations} />
                        </div>
                        {/* Empty placeholder — the actual buttons live outside the anchor as an
                            absolutely-positioned sibling, so we don't nest interactive content in <a>. */}
                        <div className={app.status.summary?.externalURLs?.length > 0 ? 'columns small-2' : 'columns small-1'} aria-hidden='true' />
                    </div>

                    <EntryFieldList variant='tile'>
                        <EntryField name='project' label='Project'>
                            {app.spec.project}
                        </EntryField>
                        <EntryField name='labels' label='Labels'>
                            <Tooltip
                                zIndex={4}
                                content={
                                    <div>
                                        {Object.keys(app.metadata.labels || {})
                                            .map(label => ({label, value: app.metadata.labels[label]}))
                                            .map(item => (
                                                <div key={item.label}>
                                                    {item.label}={item.value}
                                                </div>
                                            ))}
                                    </div>
                                }>
                                <span>
                                    {Object.keys(app.metadata.labels || {})
                                        .map(label => `${label}=${app.metadata.labels[label]}`)
                                        .join(', ')}
                                </span>
                            </Tooltip>
                        </EntryField>
                        <EntryField name='status' label='Status'>
                            <span qe-id='applications-tiles-health-status'>
                                <AppUtils.HealthStatusIcon state={app.status.health} /> {app.status.health.status}
                                &nbsp;
                                {app.status.sourceHydrator?.currentOperation && (
                                    <>
                                        <AppUtils.HydrateOperationPhaseIcon operationState={app.status.sourceHydrator.currentOperation} />{' '}
                                        {app.status.sourceHydrator.currentOperation.phase}
                                        &nbsp;
                                    </>
                                )}
                                <AppUtils.ComparisonStatusIcon status={app.status.sync.status} /> {app.status.sync.status}
                                &nbsp;
                                <OperationState app={app} quiet={true} />
                            </span>
                        </EntryField>
                        <EntryField name='repository' label='Repository'>
                            <Tooltip content={source?.repoURL || ''} zIndex={4}>
                                <span>{source?.repoURL}</span>
                            </Tooltip>
                        </EntryField>
                        <EntryField name='targetRevision' label='Target Revision'>
                            {targetRevision}
                        </EntryField>
                        {source?.path && (
                            <EntryField name='path' label='Path'>
                                {source?.path}
                            </EntryField>
                        )}
                        {source?.chart && (
                            <EntryField name='chart' label='Chart'>
                                {source?.chart}
                            </EntryField>
                        )}
                        <EntryField name='destination' label='Destination'>
                            <Cluster server={app.spec.destination.server} name={app.spec.destination.name} />
                        </EntryField>
                        <EntryField name='namespace' label='Namespace'>
                            {app.spec.destination.namespace}
                        </EntryField>
                        <EntryField name='createdAt' label='Created At'>
                            {AppUtils.formatCreationTimestamp(app.metadata.creationTimestamp)}
                        </EntryField>
                        {app.status.operationState && (
                            <EntryField name='lastSync' label='Last Sync'>
                                {AppUtils.formatCreationTimestamp(app.status.operationState.finishedAt || app.status.operationState.startedAt)}
                            </EntryField>
                        )}
                    </EntryFieldList>
                </div>
            </a>

            {/* Header buttons — sibling of the anchor (not nested) so the markup stays valid. */}
            <div className='applications-tiles__header-buttons applications-list__external-link'>
                <ApplicationURLs urls={app.status.summary?.externalURLs} />
                {managedByURLInvalid ? (
                    <button type='button' className='managed-by-url-invalid' onClick={handleExternalLinkClick} style={{cursor: 'not-allowed'}} title={MANAGED_BY_URL_INVALID_TEXT}>
                        <i className='fa fa-window-maximize' />
                    </button>
                ) : (
                    <button type='button' onClick={handleExternalLinkClick} title={managedByURL ? `Managed by: ${managedByURL}` : 'Open application'}>
                        <i className='fa fa-window-maximize' />
                    </button>
                )}
                <button title={favList?.includes(app.metadata.name) ? 'Remove Favorite' : 'Add Favorite'} className='large-text-height' onClick={handleFavoriteToggle}>
                    <i
                        className={favList?.includes(app.metadata.name) ? 'fas fa-star fa-lg' : 'far fa-star fa-lg'}
                        style={{
                            cursor: 'pointer',
                            margin: '-1px 0px 0px 0px',
                            color: favList?.includes(app.metadata.name) ? '#FFCE25' : '#8fa4b1'
                        }}
                    />
                </button>
            </div>

            {/* Action buttons — sibling of the anchor (not nested). */}
            <div className='applications-tiles__actions'>
                <button
                    type='button'
                    className='argo-button argo-button--base'
                    qe-id='applications-tiles-button-sync'
                    onClick={() => syncApplication(app.metadata.name, app.metadata.namespace)}>
                    <i className='fa fa-sync' /> Sync
                </button>
                &nbsp;
                <Tooltip className='custom-tooltip' content={'Refresh'}>
                    {/* Spreading refreshLinkAttrs (= {disabled: isAppRefreshing(app)}) onto a real
                        <button> would actively block clicks while a refresh is in flight, leaving
                        the user no way to retrigger a stuck refresh — that attribute only existed
                        because this used to be an <a>, where `disabled` is meaningless. */}
                    <button
                        type='button'
                        className='argo-button argo-button--base'
                        qe-id='applications-tiles-button-refresh'
                        onClick={() => refreshApplication(app.metadata.name, app.metadata.namespace)}>
                        <i className={classNames('fa fa-redo', {'status-icon--spin': AppUtils.isAppRefreshing(app)})} /> <span className='show-for-xxlarge'>Refresh</span>
                    </button>
                </Tooltip>
                &nbsp;
                <Tooltip className='custom-tooltip' content={'Delete'}>
                    <button
                        type='button'
                        className='argo-button argo-button--base'
                        qe-id='applications-tiles-button-delete'
                        onClick={() => deleteApplication(app.metadata.name, app.metadata.namespace)}>
                        <i className='fa fa-times-circle' /> <span className='show-for-xxlarge'>Delete</span>
                    </button>
                </Tooltip>
            </div>
        </div>
    );
};
