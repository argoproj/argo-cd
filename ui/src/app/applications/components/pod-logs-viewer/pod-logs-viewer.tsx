import {DataLoader} from 'argo-ui';
import * as React from 'react';

import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

const lines: string[] = [];

export const PodsLogsViewer = (props: {applicationName: string; pod: models.ResourceNode & any; containerIndex: number}) => {
    const containers = (props.pod.spec.initContainers || []).concat(props.pod.spec.containers || []);
    const container = containers[props.containerIndex];
    if (!container) {
        return <div>Pod does not have container with index {props.containerIndex}</div>;
    }
    const containerStatuses = ((props.pod.status && props.pod.status.containerStatuses) || []).concat((props.pod.status && props.pod.status.initContainerStatuses) || []);
    const containerStatus = containerStatuses.find((status: any) => status.name === container.name);
    const isRunning = !!(containerStatus && containerStatus.state && containerStatus && containerStatus.state.running);
    const maxLines = 10;
    return (
        <pre style={{height: '100%', lineHeight: '1em'}}>
            <DataLoader load={() => services.applications.getContainerLogs(props.applicationName, props.pod.metadata.namespace, props.pod.metadata.name, container.name, maxLines)}>
                {(log) => {
                    if (isRunning) {
                        if (lines.length >= maxLines) {
                            lines.shift();
                        }
                        lines.push(log.content);
                    }
                    return lines.map((l) => <p>{l}</p>);
                }}
            </DataLoader>
            {/* <LogsViewer
                source={{
                    key: `${props.pod.metadata.name}:${container.name}`,
                    loadLogs: () =>
                        services.applications
                            .getContainerLogs(props.applicationName, props.pod.metadata.namespace, props.pod.metadata.name, container.name)
                            .map((item) => item.content + '\n'),
                    shouldRepeat: () => isRunning,
                }}
            /> */}
        </pre>
    );
};
