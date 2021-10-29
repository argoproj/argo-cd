import {DropDownMenu} from 'argo-ui';
import * as React from 'react';
import {Key, KeybindingContext, useNav} from 'argo-ui/v2';
import {Cluster} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {OperationState} from '../utils';
import {ApplicationsLabels} from './applications-labels';
import {ApplicationsSource} from './applications-source';
require('./applications-table.scss');

export const ApplicationsTable = (props: {
    applications: models.Application[];
    syncApplication: (appName: string) => any;
    refreshApplication: (appName: string) => any;
    deleteApplication: (appName: string) => any;
}) => {
    const [selectedApp, navApp, reset] = useNav(props.applications.length);

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

    return (
        <Consumer>
            {ctx => (
                <div className='applications-table argo-table-list argo-table-list--clickable'>
                    {props.applications.map((app, i) => (
                        <div
                            key={app.metadata.name}
                            className={`argo-table-list__row
                applications-list__entry applications-list__entry--health-${app.status.health.status} ${selectedApp === i ? 'applications-tiles__selected' : ''}`}>
                            <div className={`row applications-list__table-row`} onClick={e => ctx.navigation.goto(`/applications/${app.metadata.name}`, {}, {event: e})}>
                                <div className='columns small-4'>
                                    <div className='row'>
                                        <div className='show-for-xxlarge columns small-3'>Project:</div>
                                        <div className='columns small-12 xxlarge-9'>{app.spec.project}</div>
                                    </div>
                                    <div className='row'>
                                        <div className='show-for-xxlarge columns small-3'>Name:</div>
                                        <div className='columns small-12 xxlarge-9'>
                                            {app.metadata.name} <ApplicationURLs urls={app.status.summary.externalURLs} />
                                        </div>
                                    </div>
                                </div>
                                <div className='columns small-6'>
                                    <div className='row'>
                                        <div className='show-for-xxlarge columns small-2'>Source:</div>
                                        <div className='columns small-12 xxlarge-10 applications-table-source' style={{position: 'relative'}}>
                                            <div className='applications-table-source__link'>
                                                <ApplicationsSource source={app.spec.source} />
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
                                    <AppUtils.HealthStatusIcon state={app.status.health} /> <span>{app.status.health.status}</span>
                                    <br />
                                    <AppUtils.ComparisonStatusIcon status={app.status.sync.status} />
                                    <span>{app.status.sync.status}</span> <OperationState app={app} quiet={true} />
                                    <DropDownMenu
                                        anchor={() => (
                                            <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                <i className='fa fa-ellipsis-v' />
                                            </button>
                                        )}
                                        items={[
                                            {title: 'Sync', action: () => props.syncApplication(app.metadata.name)},
                                            {title: 'Refresh', action: () => props.refreshApplication(app.metadata.name)},
                                            {title: 'Delete', action: () => props.deleteApplication(app.metadata.name)}
                                        ]}
                                    />
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </Consumer>
    );
};
