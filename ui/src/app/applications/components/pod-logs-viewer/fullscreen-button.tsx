import {Link} from 'react-router-dom';
import * as React from 'react';
import {PodLogsProps} from './pod-logs-viewer';
import {Button} from '../../../shared/components/button';

export const FullscreenButton = ({
    applicationName,
    applicationNamespace,
    containerName,
    fullscreen,
    group,
    kind,
    name,
    namespace,
    podName,
    viewPodNames,
    viewTimestamps,
    follow,
    showPreviousLogs
}: PodLogsProps & {fullscreen?: boolean}) => {
    const fullscreenURL =
        `/applications/${applicationNamespace}/${applicationName}/${namespace}/${containerName}/logs?` +
        `podName=${podName}&group=${group}&kind=${kind}&name=${name}&viewPodNames=${viewPodNames}&viewTimestamps=${viewTimestamps}&follow=${follow}&showPreviousLogs=${showPreviousLogs}`;
    return (
        !fullscreen && (
            <Link to={fullscreenURL} target='_blank' rel='noopener noreferrer'>
                <Button title='Show logs in fullscreen in a new window' icon='external-link-alt' />
            </Link>
        )
    );
};
