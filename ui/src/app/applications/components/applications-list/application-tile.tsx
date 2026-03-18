import {Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {Cluster} from '../../../shared/components';
import {ContextApis, AuthSettingsCtx} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {getAppDefaultSource, OperationState, getApplicationLinkURL, getManagedByURL} from '../utils';
import {services} from '../../../shared/services';
import {ViewPreferences} from '../../../shared/services';

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
        if (linkInfo.isExternal) {
            window.open(linkInfo.url, '_blank', 'noopener,noreferrer');
        } else {
            ctx.navigation.goto(`/${AppUtils.getAppUrl(app)}`);
        }
    };

    return (
        <div
            ref={tileRef}
            className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <div className='row applications-tiles__wrapper' onClick={e => ctx.navigation.goto(`/${AppUtils.getAppUrl(app)}`, {view: pref.appDetails.view}, {event: e})}>
                <div className={`columns small-12 applications-list__info qe-applications-list-${AppUtils.appInstanceName(app)} applications-tiles__item`}>
                    {/* Header row with icon, title, and action buttons */}
                    <div className='row'>
                        <div className={app.status.summary?.externalURLs?.length > 0 ? 'columns small-10' : 'columns small-11'}>
                            <i className={'icon argo-icon-' + (source?.chart != null ? 'helm' : isOci ? 'oci applications-tiles__item__small' : 'git')} />
                            <Tooltip content={AppUtils.appInstanceName(app)}>
                                <span className='applications-list__title'>{AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}</span>
                            </Tooltip>
                        </div>
                        <div className={app.status.summary?.externalURLs?.length > 0 ? 'columns small-2' : 'columns small-1'}>
                            <div className='applications-list__external-link'>
                                <ApplicationURLs urls={app.status.summary?.externalURLs} />
                                <button onClick={handleExternalLinkClick} title={getManagedByURL(app) ? `Managed by: ${getManagedByURL(app)}` : 'Open application'}>
                                    <i className='fa fa-external-link-alt' />
                                </button>
                                <button
                                    title={favList?.includes(app.metadata.name) ? 'Remove Favorite' : 'Add Favorite'}
                                    className='large-text-height'
                                    onClick={handleFavoriteToggle}>
                                    <i
                                        className={favList?.includes(app.metadata.name) ? 'fas fa-star fa-lg' : 'far fa-star fa-lg'}
                                        style={{
                                            cursor: 'pointer',
                                            margin: '-1px 0px 0px 7px',
                                            color: favList?.includes(app.metadata.name) ? '#FFCE25' : '#8fa4b1'
                                        }}
                                    />
                                </button>
                            </div>
                        </div>
                    </div>

                    {/* Project row */}
                    <div className='row'>
                        <div className='columns small-3' title='Project:'>
                            Project:
                        </div>
                        <div className='columns small-9'>{app.spec.project}</div>
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
                        </div>
                    </div>

                    {/* Status row */}
                    <div className='row'>
                        <div className='columns small-3' title='Status:'>
                            Status:
                        </div>
                        <div className='columns small-9' qe-id='applications-tiles-health-status'>
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
                        </div>
                    </div>

                    {/* Repository row */}
                    <div className='row'>
                        <div className='columns small-3' title='Repository:'>
                            Repository:
                        </div>
                        <div className='columns small-9'>
                            <Tooltip content={source?.repoURL || ''} zIndex={4}>
                                <span>{source?.repoURL}</span>
                            </Tooltip>
                        </div>
                    </div>

                    {/* Target Revision row */}
                    <div className='row'>
                        <div className='columns small-3' title='Target Revision:'>
                            Target Revision:
                        </div>
                        <div className='columns small-9'>{targetRevision}</div>
                    </div>

                    {/* Path row (conditional) */}
                    {source?.path && (
                        <div className='row'>
                            <div className='columns small-3' title='Path:'>
                                Path:
                            </div>
                            <div className='columns small-9'>{source?.path}</div>
                        </div>
                    )}

                    {/* Chart row (conditional) */}
                    {source?.chart && (
                        <div className='row'>
                            <div className='columns small-3' title='Chart:'>
                                Chart:
                            </div>
                            <div className='columns small-9'>{source?.chart}</div>
                        </div>
                    )}

                    {/* Destination row */}
                    <div className='row'>
                        <div className='columns small-3' title='Destination:'>
                            Destination:
                        </div>
                        <div className='columns small-9'>
                            <Cluster server={app.spec.destination.server} name={app.spec.destination.name} />
                        </div>
                    </div>

                    {/* Namespace row */}
                    <div className='row'>
                        <div className='columns small-3' title='Namespace:'>
                            Namespace:
                        </div>
                        <div className='columns small-9'>{app.spec.destination.namespace}</div>
                    </div>

                    {/* Created At row */}
                    <div className='row'>
                        <div className='columns small-3' title='Age:'>
                            Created At:
                        </div>
                        <div className='columns small-9'>{AppUtils.formatCreationTimestamp(app.metadata.creationTimestamp)}</div>
                    </div>

                    {/* Last Sync row (conditional) */}
                    {app.status.operationState && (
                        <div className='row'>
                            <div className='columns small-3' title='Last sync:'>
                                Last Sync:
                            </div>
                            <div className='columns small-9'>{AppUtils.formatCreationTimestamp(app.status.operationState.finishedAt || app.status.operationState.startedAt)}</div>
                        </div>
                    )}

                    {/* Action buttons */}
                    <div className='row applications-tiles__actions'>
                        <div className='columns applications-list__entry--actions'>
                            <a
                                className='argo-button argo-button--base'
                                qe-id='applications-tiles-button-sync'
                                onClick={e => {
                                    e.stopPropagation();
                                    syncApplication(app.metadata.name, app.metadata.namespace);
                                }}>
                                <i className='fa fa-sync' /> Sync
                            </a>
                            &nbsp;
                            <Tooltip className='custom-tooltip' content={'Refresh'}>
                                <a
                                    className='argo-button argo-button--base'
                                    qe-id='applications-tiles-button-refresh'
                                    {...AppUtils.refreshLinkAttrs(app)}
                                    onClick={e => {
                                        e.stopPropagation();
                                        refreshApplication(app.metadata.name, app.metadata.namespace);
                                    }}>
                                    <i className={classNames('fa fa-redo', {'status-icon--spin': AppUtils.isAppRefreshing(app)})} />{' '}
                                    <span className='show-for-xxlarge'>Refresh</span>
                                </a>
                            </Tooltip>
                            &nbsp;
                            <Tooltip className='custom-tooltip' content={'Delete'}>
                                <a
                                    className='argo-button argo-button--base'
                                    qe-id='applications-tiles-button-delete'
                                    onClick={e => {
                                        e.stopPropagation();
                                        deleteApplication(app.metadata.name, app.metadata.namespace);
                                    }}>
                                    <i className='fa fa-times-circle' /> <span className='show-for-xxlarge'>Delete</span>
                                </a>
                            </Tooltip>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};
