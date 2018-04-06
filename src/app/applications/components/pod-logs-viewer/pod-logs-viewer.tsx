import { LogsViewer } from 'argo-ui';
import * as React from 'react';

import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

export const PodsLogsViewer = (props: { applicationName: string, pod: models.ResourceNode & any }) => {
    const container = props.pod.spec.containers[0];
    const isRunning = props.pod.status.phase === 'Running';
    return (
        <div style={{height: '100%'}}>
            <LogsViewer source={{
                key: props.pod.metadata.name,
                loadLogs: () => services.applications.getContainerLogs(
                    props.applicationName, props.pod.metadata.name, container.name).map((item) => item.content + '\n'),
                shouldRepeat: () => isRunning,
            }} />
        </div>
    );
};
