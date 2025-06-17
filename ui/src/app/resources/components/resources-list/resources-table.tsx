import {DataLoader, Tooltip} from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
import {Key, KeybindingContext, useNav} from 'argo-ui/v2';
import {Cluster} from '../../../shared/components';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import * as AppUtils from '../../../applications/components/utils';
import './resources-table.scss';

export const ResourcesTable = (props: {resources: models.Resource[]}) => {
    const [selectedResource, navResource, reset] = useNav(props.resources.length);
    const ctxh = React.useContext(Context);

    const {useKeybinding} = React.useContext(KeybindingContext);

    useKeybinding({keys: Key.DOWN, action: () => navResource(1)});
    useKeybinding({keys: Key.UP, action: () => navResource(-1)});
    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            reset();
            return selectedResource > -1 ? true : false;
        }
    });
    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedResource > -1) {
                ctxh.navigation.goto(
                    AppUtils.getAppUrl({
                        metadata: {
                            name: props.resources[selectedResource].appName,
                            namespace: props.resources[selectedResource].namespace
                        }
                    } as models.Application)
                );
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
                        return (
                            <div className='resources-table argo-table-list argo-table-list--clickable'>
                                {props.resources.map((resource, i) => (
                                    <div
                                        key={`${resource.appProject}/${resource.appName}/${resource.name}/${resource.namespace}/${resource.name}/${resource.kind}/${resource.group}/${resource.version}`}
                                        className={`argo-table-list__row resources-list__entry resources-list__entry--health-${resource.health?.status} ${selectedResource === i ? 'resources-list__selected' : ''}`}>
                                        <div
                                            className={`row resources-list__table-row`}
                                            onClick={e =>
                                                ctx.navigation.goto(
                                                    AppUtils.getAppUrl({
                                                        metadata: {
                                                            name: resource.appName,
                                                            namespace: resource.appNamespace
                                                        }
                                                    } as models.Application),
                                                    {
                                                        view: pref.appDetails.view,
                                                        node: `/${resource.kind}${resource.appNamespace ? `/${resource.appNamespace}` : ''}/${resource.name}/0`
                                                    },
                                                    {event: e}
                                                )
                                            }>
                                            <div className='columns small-4'>
                                                <div className='row'>
                                                    <div className='show-for-xxlarge columns small-4'>Project:</div>
                                                    <div className='columns small-12 xxlarge-6'>{resource.appProject}</div>
                                                </div>
                                                <div className='row'>
                                                    <div className='show-for-xxlarge columns small-4'>Name:</div>
                                                    <div className='columns small-12 xxlarge-6'>
                                                        <Tooltip
                                                            content={
                                                                <>
                                                                    {resource.name}
                                                                    <br />
                                                                    <Moment fromNow={true} ago={true}>
                                                                        {resource.createdAt}
                                                                    </Moment>
                                                                </>
                                                            }>
                                                            <span>{resource.name}</span>
                                                        </Tooltip>
                                                    </div>
                                                </div>
                                            </div>

                                            <div className='columns small-4'>
                                                <div className='row'>
                                                    <div className='show-for-xxlarge columns small-4'>Application:</div>
                                                    <div className='columns small-12 xxlarge-6'>{resource.appName}</div>
                                                </div>
                                                <div className='row'>
                                                    <div className='show-for-xxlarge columns small-4'>Destination:</div>
                                                    <div className='columns small-12 xxlarge-6'>
                                                        <Cluster server={resource.clusterServer} name={resource.clusterName} />/{resource.namespace}
                                                    </div>
                                                </div>
                                            </div>

                                            <div className='columns small-2'>
                                                {resource?.health && (
                                                    <>
                                                        <AppUtils.HealthStatusIcon state={resource?.health} /> {resource?.health?.status} <br />
                                                    </>
                                                )}
                                                {resource?.status && (
                                                    <>
                                                        <AppUtils.ComparisonStatusIcon status={resource?.status} />
                                                        <span>{resource.status}</span>
                                                    </>
                                                )}
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
