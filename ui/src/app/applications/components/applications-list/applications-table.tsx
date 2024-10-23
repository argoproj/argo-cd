import {DataLoader, DropDownMenu, Tooltip} from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
import {Key, KeybindingContext, useNav} from 'argo-ui/v2';
import {Cluster} from '../../../shared/components';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {getAppDefaultSource, OperationState} from '../utils';
import {ApplicationsLabels} from './applications-labels';
import {ApplicationsSource} from './applications-source';
import {services} from '../../../shared/services';
import './applications-table.scss';

export const ApplicationsTable = (props: {
    applications: models.Application[];
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
                ctxh.navigation.goto(`/applications/${props.applications[selectedApp].metadata.name}`);
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
                                {props.applications.map((app, i) => (
                                    <div
                                        key={AppUtils.appInstanceName(app)}
                                        className={`argo-table-list__row
                applications-list__entry applications-list__entry--health-${app.status.health.status} ${selectedApp === i ? 'applications-tiles__selected' : ''}`}>
                                        <div
                                            className={`row applications-list__table-row`}
                                            onClick={e => ctx.navigation.goto(`applications/${app.metadata.namespace}/${app.metadata.name}`, {}, {event: e})}>
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
                                                            <ApplicationURLs urls={app.status.summary.externalURLs} />
                                                        </div>
                                                    </div>
                                                    <div className='show-for-xxlarge columns small-4'>Project:</div>
                                                    <div className='columns small-12 xxlarge-6'>{app.spec.project}</div>
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
                                                    </div>
                                                </div>
                                            </div>

                                            <div className='columns small-6'>
                                                <div className='row'>
                                                    <div className='show-for-xxlarge columns small-2'>Source:</div>
                                                    <div className='columns small-12 xxlarge-10 applications-table-source' style={{position: 'relative'}}>
                                                        <div className='applications-table-source__link'>
                                                            <ApplicationsSource source={getAppDefaultSource(app)} />
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

                                            <div className='columns small-2'>
                                                <AppUtils.HealthStatusIcon state={app.status.health} /> <span>{app.status.health.status}</span> <br />
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
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        );
                    }}
                </DataLoader>
            )}
        </Consumer>
    );
};
