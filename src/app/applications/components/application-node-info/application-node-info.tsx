import * as React from 'react';

import * as models from '../../../shared/models';

import { ApplicationResourceDiff } from '../application-resource-diff/application-resource-diff';
import { ComparisonStatusIcon, getPodPhase, getStateAndNode } from '../utils';

require('./application-node-info.scss');

export const ApplicationNodeInfo = (props: { node: models.ResourceNode | models.ResourceState}) => {
    const {resourceNode, resourceState} = getStateAndNode(props.node);

    const attributes = [
        {title: 'KIND', value: resourceNode.state.kind},
        {title: 'NAME', value: resourceNode.state.metadata.name},
        {title: 'NAMESPACE', value: resourceNode.state.metadata.namespace},
    ];
    if (resourceNode.state.kind === 'Pod') {
        attributes.push({title: 'PHASE', value: getPodPhase(resourceNode.state)});
    } else if (resourceNode.state.kind === 'Service') {
        attributes.push({title: 'TYPE', value: resourceNode.state.spec.type});
        let hostNames = '';
        const status = resourceNode.state.status;
        if (status && status.loadBalancer && status.loadBalancer.ingress) {
            hostNames = (status.loadBalancer.ingress || []).map((item: any) => item.hostname).join(', ');
        }
        attributes.push({title: 'HOSTNAMES', value: hostNames});
    }
    if (resourceState) {
        attributes.push({title: 'STATUS', value: (
            <span><ComparisonStatusIcon status={resourceState.status}/> {resourceState.status}</span>
        )} as any);
    }

    return (
        <div>
            <div className='white-box'>
                <div className='white-box__details'>
                    {attributes.map((attr) => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>
                                {attr.title}
                            </div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>

            {resourceState &&
                <div className='application-node-info__manifest'>
                    <ApplicationResourceDiff targetState={resourceState.targetState} liveState={resourceState.liveState}/>
                </div>
            }
        </div>
    );
};
