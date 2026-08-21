import {services} from '../../../shared/services';
import * as React from 'react';
import {PodLogsProps} from './pod-logs-viewer';
import {Button} from '../../../shared/components/button';

interface DownloadLogsButtonProps extends PodLogsProps {
    previous?: boolean;
}

// DownloadLogsButton is a button that downloads the logs to a file
export const DownloadLogsButton = ({applicationName, applicationNamespace, containerName, group, kind, name, namespace, podName, previous}: DownloadLogsButtonProps) => (
    <Button
        title='Download logs to file'
        icon='download'
        onClick={async () => {
            const downloadURL = services.applications.getDownloadLogsURL(applicationName, applicationNamespace, namespace, podName, {group, kind, name}, containerName, previous);
            window.open(downloadURL, '_blank');
        }}
    />
);
