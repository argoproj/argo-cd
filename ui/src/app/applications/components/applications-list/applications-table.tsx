import {DataLoader, DropDownMenu, Tooltip} from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
import {Key, KeybindingContext, useNav} from 'argo-ui/v2';
import {Cluster} from '../../../shared/components';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {getAppDefaultSource, OperationState, getApplicationLinkURL, getManagedByURL, isApp, getAppSetHealthStatus} from '../utils';
import {ApplicationsLabels} from './applications-labels';
import {ApplicationsSource} from './applications-source';
import {services} from '../../../shared/services';
import './applications-table.scss';

export const ApplicationsTable = (props: {
    applications: models.AbstractApplication[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
    deleteApplication: (appName: string, appNamespace: string) => any;
}) => {
    const [selectedApp, navApp, reset] = useNav(props.applications.length);
    const ctxh = React.useContext(Context);

    const {useKeybinding} = React.useContext(KeybindingContext);

    useKeybinding({keys: Key.DOWN, action: () => navApp(1)});
    useKeybinding({keys: Key.UP, action: () => navApp(-1)});
    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            reset();
            return selectedApp > -1 ? true : false;
        }
    });
    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedApp > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(props.applications[selectedApp])}`);
                return true;
            }
            return false;
        }
    });

    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        const favList = pref.appList.favoritesAppList || [];
                        return (
                            <div className='applications-table argo-table-list argo-table-list--clickable'>
                                {props.applications.map((app, i) => {
                                    const isApplication = isApp(app);
                                    const typedApp = isApplication ? (app as models.Application) : null;
                                    const typedAppSet = !isApplication ? (app as models.ApplicationSet) : null;
                                    const healthStatus = isApplication ? typedApp.status.health.status : getAppSetHealthStatus(typedAppSet);
                                    return (
                                        <div
                                            key={AppUtils.appInstanceName(app)}
                                            className={`argo-table-list__row
                applications-list__entry applications-list__entry--health-${healthStatus} ${selectedApp === i ? 'applications-tiles__selected' : ''}`}>
                                            <div
                                                className={`row applications-list__table-row ${isApplication && typedApp.status.sourceHydrator?.currentOperation ? 'applications-table-row--with-hydrator' : ''}`}
                                                onClick={e => ctx.navigation.goto(`/${AppUtils.getAppUrl(app)}`, {}, {event: e})}>
                                                <div className='columns small-4'>
                                                    <div className='row'>
                                                        <div className=' columns small-2'>
                                                            <div>
                                                                <Tooltip content={favList?.includes(app.metadata.name) ? 'Remove Favorite' : 'Add Favorite'}>
                                                                    <button
                                                                        onClick={e => {
                                                                            e.stopPropagation();
                                                                            favList?.includes(app.metadata.name)
                                                                                ? favList.splice(favList.indexOf(app.metadata.name), 1)
                                                                                : favList.push(app.metadata.name);
                                                                            services.viewPreferences.updatePreferences({appList: {...pref.appList, favoritesAppList: favList}});
                                                                        }}>
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
                                                                {isApplication && <ApplicationURLs urls={typedApp.status.summary?.externalURLs} />}
                                                            </div>
                                                        </div>
                                                        <div className='show-for-xxlarge columns small-4'>{isApplication ? 'Project:' : 'Kind:'}</div>
                                                        <div className='columns small-12 xxlarge-6'>{isApplication ? typedApp.spec.project : 'ApplicationSet'}</div>
                                                    </div>
                                                    <div className='row'>
                                                        <div className=' columns small-2' />
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
                                                            {/* External link icon for managed-by-url */}
                                                            {(() => {
                                                                const linkInfo = getApplicationLinkURL(app, ctx.baseHref);
                                                                return (
                                                                    <button
                                                                        onClick={e => {
                                                                            e.stopPropagation();
                                                                            if (linkInfo.isExternal) {
                                                                                window.open(linkInfo.url, '_blank', 'noopener,noreferrer');
                                                                            } else {
                                                                                ctx.navigation.goto(`/${AppUtils.getAppUrl(app)}`);
                                                                            }
                                                                        }}
                                                                        style={{marginLeft: '0.5em'}}
                                                                        title={`Link: ${linkInfo.url}\nmanaged-by-url: ${getManagedByURL(app) || 'none'}`}>
                                                                        <i className='fa fa-external-link-alt' />
                                                                    </button>
                                                                );
                                                            })()}
                                                        </div>
                                                    </div>
                                                </div>

                                                {isApplication && typedApp && (
                                                    <div className='columns small-6'>
                                                        <div className='row'>
                                                            <div className='show-for-xxlarge columns small-2'>Source:</div>
                                                            <div className='columns small-12 xxlarge-10 applications-table-source' style={{position: 'relative'}}>
                                                                <div className='applications-table-source__link'>
                                                                    <ApplicationsSource source={getAppDefaultSource(typedApp)} />
                                                                </div>
                                                                <div className='applications-table-source__labels'>
                                                                    <ApplicationsLabels app={typedApp} />
                                                                </div>
                                                            </div>
                                                        </div>
                                                        <div className='row'>
                                                            <div className='show-for-xxlarge columns small-2'>Destination:</div>
                                                            <div className='columns small-12 xxlarge-10'>
                                                                <Cluster server={typedApp.spec.destination.server} name={typedApp.spec.destination.name} />/
                                                                {typedApp.spec.destination.namespace}
                                                            </div>
                                                        </div>
                                                    </div>
                                                )}

                                                <div className={isApplication ? 'columns small-2' : 'columns small-8'}>
                                                    {isApplication && typedApp && (
                                                        <>
                                                            <AppUtils.HealthStatusIcon state={typedApp.status.health} /> <span>{typedApp.status.health.status}</span> <br />
                                                            {typedApp.status.sourceHydrator?.currentOperation && (
                                                                <>
                                                                    <AppUtils.HydrateOperationPhaseIcon operationState={typedApp.status.sourceHydrator.currentOperation} />{' '}
                                                                    <span>{typedApp.status.sourceHydrator.currentOperation.phase}</span> <br />
                                                                </>
                                                            )}
                                                            <AppUtils.ComparisonStatusIcon status={typedApp.status.sync.status} />
                                                            <span>{typedApp.status.sync.status}</span> <OperationState app={typedApp} quiet={true} />
                                                        </>
                                                    )}
                                                    {!isApplication && typedAppSet && (
                                                        <>
                                                            <AppUtils.HealthStatusIcon state={{status: getAppSetHealthStatus(typedAppSet), message: ''}} />{' '}
                                                            <span>{getAppSetHealthStatus(typedAppSet)}</span>
                                                        </>
                                                    )}
                                                    {isApplication && (
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
                                                                    action: () => props.syncApplication(app.metadata.name, app.metadata.namespace)
                                                                },
                                                                {
                                                                    title: 'Refresh',
                                                                    iconClassName: 'fa fa-fw fa-redo',
                                                                    action: () => props.refreshApplication(app.metadata.name, app.metadata.namespace)
                                                                },
                                                                {
                                                                    title: 'Delete',
                                                                    iconClassName: 'fa fa-fw fa-times-circle',
                                                                    action: () => props.deleteApplication(app.metadata.name, app.metadata.namespace)
                                                                }
                                                            ]}
                                                        />
                                                    )}
                                                </div>
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>
                        );
                    }}
                </DataLoader>
            )}
        </Consumer>
    );
};
