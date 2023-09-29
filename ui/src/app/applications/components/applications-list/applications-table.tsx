import { DataLoader, DropDownMenu, Tooltip } from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
import { Key, KeybindingContext, useNav } from 'argo-ui/v2';
import { Cluster } from '../../../shared/components';
import { Consumer, Context } from '../../../shared/context';
import * as models from '../../../shared/models';
import { ApplicationURLs } from '../application-urls';
import * as AppUtils from '../utils';
import { getAppDefaultSource, getAppSetHealthStatus, isApp, OperationState } from '../utils';
import { ApplicationsLabels } from './applications-labels';
import { ApplicationsSource } from './applications-source';
import { services } from '../../../shared/services';
import './applications-table.scss';
import { ApplicationSet } from '../../../shared/models';

export const ApplicationsTable = (props: {
    applications: models.AbstractApplication[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
    deleteApplication: (appName: string, appNamespace: string) => any;
}) => {
    const [selectedApp, navApp, reset] = useNav(props.applications.length);
    const ctxh = React.useContext(Context);

    const { useKeybinding } = React.useContext(KeybindingContext);

    useKeybinding({ keys: Key.DOWN, action: () => navApp(1) });
    useKeybinding({ keys: Key.UP, action: () => navApp(-1) });
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
                ctxh.navigation.goto(`${AppUtils.getRootPath()}/${props.applications[selectedApp].metadata.name}`);
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
                applications-list__entry applications-list__entry--health-${isApp(app) ? (app as models.Application).status.health.status : getAppSetHealthStatus((app as ApplicationSet).status)} ${selectedApp === i ? 'applications-tiles__selected' : ''}`}>
                                        <div
                                            className={`row applications-list__table-row`}
                                            onClick={e => ctx.navigation.goto(`${AppUtils.getRootPath()}/${app.metadata.namespace}/${app.metadata.name}`, {}, { event: e })}>
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
                                                                        services.viewPreferences.updatePreferences({ appList: { ...pref.appList, favoritesAppList: favList } });
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
                                                            {isApp(app) && <ApplicationURLs urls={(app as models.Application).status.summary.externalURLs} />}
                                                        </div>
                                                    </div>
                                                    {isApp(app) && <div className='show-for-xxlarge columns small-4'>Project:</div>}
                                                    {isApp(app) && <div className='columns small-12 xxlarge-6'>{(app as models.Application).spec.project}</div>}
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

                                            {isApp(app) && (<div className='columns small-6'>
                                                <div className='row'>
                                                    <div className='show-for-xxlarge columns small-2'>Source:</div>
                                                    <div className='columns small-12 xxlarge-10 applications-table-source' style={{ position: 'relative' }}>
                                                        <div className='applications-table-source__link'>
                                                            <ApplicationsSource source={getAppDefaultSource(app as models.Application)} />
                                                        </div>
                                                        <div className='applications-table-source__labels'>
                                                            <ApplicationsLabels app={(app as models.Application)} />
                                                        </div>
                                                    </div>
                                                </div>
                                                <div className='row'>
                                                    <div className='show-for-xxlarge columns small-2'>Destination:</div>
                                                    <div className='columns small-12 xxlarge-10'>
                                                        <Cluster server={(app as models.Application).spec.destination.server} name={(app as models.Application).spec.destination.name} />/{(app as models.Application).spec.destination.namespace}
                                                    </div>
                                                </div>
                                            </div>
                                            )}
                                            <div className='columns small-2'>
                                                {isApp(app) && <AppUtils.HealthStatusIcon state={(app as models.Application).status.health} />} {isApp(app) && <span>{(app as models.Application).status.health.status}</span>} {isApp(app) &&  <br />}
                                                {!isApp(app) && <AppUtils.AppSetHealthStatusIcon state={(app as models.ApplicationSet).status} />} {!isApp(app) && <span>{getAppSetHealthStatus((app as ApplicationSet).status)}</span>} {!isApp(app) &&  <br />}
                                                {isApp(app) && <AppUtils.ComparisonStatusIcon status={(app as models.Application).status.sync.status} />}
                                                {isApp(app) && <span>{(app as models.Application).status.sync.status}</span>} {isApp(app) && <OperationState app={(app as models.Application)} quiet={true} />}
                                                <DropDownMenu
                                                    anchor={() => (
                                                        <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                            <i className='fa fa-ellipsis-v' />
                                                        </button>
                                                    )}
                                                    items={isApp(app) ? [
                                                        { title: 'Sync', action: () => props.syncApplication(app.metadata.name, app.metadata.namespace) },
                                                        { title: 'Refresh', action: () => props.refreshApplication(app.metadata.name, app.metadata.namespace) },
                                                        { title: 'Delete', action: () => props.deleteApplication(app.metadata.name, app.metadata.namespace) }
                                                    ] :
                                                    [{ title: 'Delete', action: () => props.deleteApplication(app.metadata.name, app.metadata.namespace) }]}
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
