import React, {useState, useEffect} from 'react';
import {services} from '../../../shared/services';
import {YamlEditor} from '../../../shared/components';
import {DataLoader} from 'argo-ui';
import {AppContext} from '../../../shared/context';
import * as PropTypes from 'prop-types';
import * as models from '../../../shared/models';

import './resource-state-overview.scss';
import {nodeKey} from '../utils';

export const ResourceStateOverview = ({app, treeNodes}: {app: models.Application; treeNodes: models.ResourceNode[]}, ctx: AppContext) => {
    const [expandedResourceStatus, setExpandedResourceStatus] = useState<string | null>(null);
    const [resourceEventsCount, setResourceEventsCount] = useState(new Map<string, number>());

    useEffect(() => {
        if (treeNodes) {
            const fetchResourceEvents = async () => {
                const eventsPromises = treeNodes
                    .filter(rNode => rNode?.health?.status === 'Degraded' || rNode?.health?.status === 'Unknown')
                    .map(res => {
                        return services.applications
                            .resourceEvents(app.metadata.name, app.metadata.namespace, {
                                namespace: res.namespace,
                                name: res.name,
                                uid: res.uid
                            } as models.ResourceNode)
                            .then(events => {
                                const eventCount = events
                                    .filter(resEvent => resEvent.involvedObject.uid === res.uid && resEvent.type !== 'Normal')
                                    .reduce((total, event) => total + event.count, 0);
                                return {resName: res.name, eventCount};
                            });
                    });

                const eventsCount = await Promise.all(eventsPromises);
                eventsCount.forEach(event => {
                    resourceEventsCount.set(event.resName, event.eventCount);
                });
                setResourceEventsCount(resourceEventsCount);
            };

            fetchResourceEvents();
        }
    }, [treeNodes, app]);

    const fetchResourceStatus = async (res?: models.ResourceNode, type?: string) => {
        try {
            let state = null;

            if (type === 'status') {
                state = await services.applications.getResource(app.metadata.name, app.metadata.namespace, res);
            }

            return {res, state};
        } catch (error) {
            console.log('Error fetching resource data:', error);
            return {res, state: null};
        }
    };

    const expandResourceStatus = (resName: string) => {
        setExpandedResourceStatus(prevResource => (prevResource === resName ? null : resName));
    };

    return (
        <div className='resource-state-overview'>
            {
                <div className='argo-table-list'>
                    <div className='argo-table-list__head'>
                        <div className='row'>
                            <div className='columns small-2 xxlarge-2'>RESOURCE</div>
                            <div className='columns small-1 xxlarge-1'>KIND</div>
                            <div className='columns small-2 xxlarge-2'>HEALTH STATUS</div>
                            <div className='columns small-2 xxlarge-2'>HEALTH DETAILS</div>
                            <div className='columns small-2 xxlarge-2'>RESOURCE STATUS</div>
                            <div className='columns small-2 xxlarge-2'>EVENTS</div>
                        </div>
                    </div>
                    {treeNodes &&
                        treeNodes
                            .filter(rNode => rNode?.health?.status === 'Degraded' || rNode?.health?.status === 'Unknown')
                            .map(res => (
                                <div key={res.uid} className={`argo-table-list__row resource-state-overview__lineitem resource-state-overview__lineitem--${expandedResourceStatus === res.name ? 'Error' : 'Gray'}`}>
                                    <div className='row'>
                                        <div className='columns small-2 xxlarge-2'>{res?.name}</div>
                                        <div className='columns small-1 xxlarge-1'>{res?.kind}</div>
                                        <div className='columns small-2 xxlarge-2'>{res?.health.status}</div>
                                        <div className='columns small-2 xxlarge-2'>
                                            {res?.health?.message}
                                        </div>
                                        <div className='columns small-2 xxlarge-2'>
                                            {expandedResourceStatus != res.name && (
                                                <button onClick={() => expandResourceStatus(res.name)} className='argo-button argo-button--base-o'>
                                                    Resource Status
                                                </button>
                                            )}
                                        </div>
                                        <div className='columns small-2 xxlarge-2'>
                                            {resourceEventsCount.get(res.name) > 0 ? (
                                                <React.Fragment>
                                                    <div style={{color: '#ff6262'}}>
                                                        {resourceEventsCount.get(res.name)} Events &nbsp;
                                                        <a
                                                            className='fa-solid fa-circle-dot fa-fade'
                                                            onClick={() =>
                                                                ctx.apis.navigation.goto('.', {node: nodeKey(res), resourceState: 'false', tab: 'events'}, {replace: true})
                                                            }></a>
                                                    </div>
                                                </React.Fragment>
                                            ) : (
                                                'No events available'
                                            )}
                                        </div>
                                    </div>
                                    {expandedResourceStatus === res.name && (
                                        <div className='row'>
                                            <div className='columns small-12 xxlarge-10'>
                                                <DataLoader load={() => fetchResourceStatus(res, 'status')}>
                                                    {({state}) => <YamlEditor input={state?.status} hideModeButtons={true} />}
                                                </DataLoader>
                                            </div>
                                            <span>
                                                <i className='argo-icon-close' onClick={() => expandResourceStatus(res.name)} />
                                            </span>
                                        </div>
                                    )}
                                </div>
                            ))}
                </div>
            }
        </div>
    );
};

ResourceStateOverview.contextTypes = {
    apis: PropTypes.object
};
