import { LogsViewer } from 'argo-ui';
import * as React from 'react';

import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

export const PodsLogsViewer = (props: { applicationName: string, pod: models.ResourceNode & any, containerIndex: number }) => {
    const containers = (props.pod.spec.initContainers || []).concat(props.pod.spec.containers || []);
    const container = containers[props.containerIndex];
    if (!container) {
        return <div>Pod does not have container with index {props.containerIndex}</div>;
    }
    const containerStatuses = (props.pod.status && props.pod.status.containerStatuses || []).concat(props.pod.status && props.pod.status.initContainerStatuses || []);
    const containerStatus = containerStatuses.find((status: any) => status.name === container.name);
    const isRunning = !!(containerStatus && containerStatus.state && containerStatus && containerStatus.state.running);
    return (
        <div style={{height: '100%'}}>
            <LogsViewer source={{
                key: `${props.pod.metadata.name}:${container.name}`,
                loadLogs: () => services.applications.getContainerLogs(
                    props.applicationName, props.pod.metadata.namespace, props.pod.metadata.name, container.name).map((item) => item.content + '\n'),
                shouldRepeat: () => isRunning,
            }} />
        </div>
    );
};
