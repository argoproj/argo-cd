import { LogsViewer } from 'argo-ui';
import * as React from 'react';

import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

export const PodsLogsViewer = (props: { applicationName: string, pod: models.ResourceNode & any, containerIndex: number }) => {
    const container = props.pod.spec.containers[props.containerIndex];
    const isRunning = props.pod.status.phase === 'Running';
    return (
        <div style={{height: '100%'}}>
            <LogsViewer source={{
                key: `${props.pod.metadata.name}:${container.name}`,
                loadLogs: () => services.applications.getContainerLogs(
                    props.applicationName, props.pod.metadata.name, container.name).map((item) => item.content + '\n'),
                shouldRepeat: () => isRunning,
            }} />
        </div>
    );
};
