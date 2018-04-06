import * as React from 'react';

import * as models from '../../../shared/models';

import { ApplicationResourceDiff } from '../application-resource-diff/application-resource-diff';

require('./application-node-info.scss');

export const ApplicationNodeInfo = (props: { node: models.ResourceNode | models.ResourceState}) => {
    let resourceNode: models.ResourceNode;
    let resourceState = props.node as models.ResourceState;
    if (resourceState.liveState || resourceState.targetState) {
        resourceNode = { state: resourceState.liveState || resourceState.targetState, children: resourceState.childLiveResources };
    } else {
        resourceState = null;
        resourceNode = props.node as models.ResourceNode;
    }

    const attributes = [
        {title: 'KIND', value: resourceNode.state.kind},
        {title: 'NAME', value: resourceNode.state.metadata.name},
        {title: 'NAMESPACE', value: resourceNode.state.metadata.namespace},
    ];
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
