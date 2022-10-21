import {Tooltip} from "argo-ui";
import {services} from "../../../shared/services";
import * as React from "react";
import {PodLogsProps} from "./pod-logs-viewer";

export const DownloadLogsButton = ({
                                       applicationName,
                                       applicationNamespace,
                                       containerName,
                                       group,
                                       kind,
                                       name,
                                       namespace,
                                       podName
                                   }: PodLogsProps) =>
    <Tooltip content='Download logs'>
        <button
            className='argo-button argo-button--base'
            onClick={async () => {
                const downloadURL = services.applications.getDownloadLogsURL(
                    applicationName,
                    applicationNamespace,
                    namespace,
                    podName,
                    {group: group, kind: kind, name: name},
                    containerName
                );
                window.open(downloadURL, '_blank');
            }}>
            <i className='fa fa-download'/>
        </button>
    </Tooltip>