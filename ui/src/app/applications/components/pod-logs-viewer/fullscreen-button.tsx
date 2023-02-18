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
    podName
}: PodLogsProps & {fullscreen?: boolean}) => {
    const fullscreenURL =
        `/applications/${applicationNamespace}/${applicationName}/${namespace}/${containerName}/logs?` + `podName=${podName}&group=${group}&kind=${kind}&name=${name}`;
    return (
        !fullscreen && (
            <Link to={fullscreenURL} target='_blank'>
                <Button title='Show logs in fullscreen in a new window' icon='external-link-alt' />
            </Link>
        )
    );
};
