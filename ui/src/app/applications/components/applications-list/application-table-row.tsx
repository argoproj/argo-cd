import {DropDownMenu, Tooltip} from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
import {Cluster} from '../../../shared/components';
import {ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {getAppDefaultSource, OperationState, getApplicationLinkURL, getManagedByURL} from '../utils';
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

export const ApplicationTableRow = ({app, selected, pref, ctx, syncApplication, refreshApplication, deleteApplication}: ApplicationTableRowProps) => {
    const favList = pref.appList.favoritesAppList || [];
    const healthStatus = app.status.health.status;
    const linkInfo = getApplicationLinkURL(app, ctx.baseHref);
    const source = getAppDefaultSource(app);

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
        <div className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${selected ? 'applications-tiles__selected' : ''}`}>
            <div
                className={`row applications-list__table-row ${app.status.sourceHydrator?.currentOperation ? 'applications-table-row--with-hydrator' : ''}`}
                onClick={e => ctx.navigation.goto(`/${AppUtils.getAppUrl(app)}`, {}, {event: e})}>
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
                                <span>{app.metadata.name}</span>
                            </Tooltip>
                            <button
                                onClick={handleExternalLinkClick}
                                style={{marginLeft: '0.5em'}}
                                title={`Link: ${linkInfo.url}\nmanaged-by-url: ${getManagedByURL(app) || 'none'}`}>
                                <i className='fa fa-external-link-alt' />
                            </button>
                        </div>
                    </div>
                </div>

                {/* Second column: Source and Destination */}
                <div className='columns small-6'>
                    <div className='row'>
                        <div className='show-for-xxlarge columns small-2'>Source:</div>
                        <div className='columns small-12 xxlarge-10 applications-table-source' style={{position: 'relative'}}>
                            <div className='applications-table-source__link'>
                                <ApplicationsSource source={source} />
                            </div>
                            <div className='applications-table-source__labels'>
                                <ApplicationsLabels app={app} />
                            </div>
                        </div>
                    </div>
                    <div className='row'>
                        <div className='show-for-xxlarge columns small-2'>Destination:</div>
                        <div className='columns small-12 xxlarge-10'>
                            <Cluster server={app.spec.destination.server} name={app.spec.destination.name} />/{app.spec.destination.namespace}
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
                                title: 'Diff',
                                iconClassName: 'fa fa-fw fa-file-medical',
                                action: () => {
                                    if (app.status.sync.status !== models.SyncStatuses.Synced) {
                                        ctx.navigation.goto(`/${AppUtils.getAppUrl(app)}`, {
                                            node:
                                                AppUtils.nodeKey({
                                                    group: 'argoproj.io',
                                                    kind: 'Application',
                                                    name: app.metadata.name,
                                                    namespace: app.metadata.namespace
                                                }) + '/0',
                                            tab: 'diff'
                                        });
                                    }
                                }
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
