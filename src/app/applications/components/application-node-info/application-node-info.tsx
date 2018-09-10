import { Tab, Tabs } from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';

import * as models from '../../../shared/models';
import { ApplicationResourceDiff } from '../application-resource-diff/application-resource-diff';
import { ComparisonStatusIcon, getPodStateReason, getStateAndNode, HealthStatusIcon } from '../utils';

require('./application-node-info.scss');

export const ApplicationNodeInfo = (props: { node: models.ResourceNode | models.ResourceState}) => {
    const {resourceNode, resourceState} = getStateAndNode(props.node);

    const attributes = [
        {title: 'KIND', value: resourceNode.state.kind},
        {title: 'NAME', value: resourceNode.state.metadata.name},
        {title: 'NAMESPACE', value: resourceNode.state.metadata.namespace},
    ];
    if (resourceNode.state.kind === 'Pod') {
        const {reason, message} = getPodStateReason(resourceNode.state);
        attributes.push({title: 'STATE', value: reason });
        if (message) {
            attributes.push({title: 'STATE DETAILS', value: message });
        }
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
        attributes.push({title: 'HEALTH', value: (
            <span><HealthStatusIcon state={resourceState.health}/> {resourceState.health.status}</span>
        )} as any);
        if (resourceState.health.statusDetails) {
            attributes.push({title: 'HEALTH DETAILS', value: resourceState.health.statusDetails});
        }
    }

    const tabs: Tab[] = [{
        key: 'manifest',
        title: 'Manifest',
        content: <div className='application-node-info__manifest application-node-info__manifest--raw'>{jsYaml.safeDump(resourceNode.state, {indent: 2 })}</div>,
    }];
    if (resourceState) {
        tabs.unshift({
            key: 'diff',
            title: 'Diff',
            content: <ApplicationResourceDiff targetState={resourceState.targetState} liveState={resourceState.liveState}/>,
        });
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

            <div className='application-node-info__manifest'>
                <Tabs selectedTabKey={tabs[0].key} tabs={tabs} />
            </div>
        </div>
    );
};
