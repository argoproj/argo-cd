import {Tooltip} from "argo-ui";
import {Link} from "react-router-dom";
import * as React from "react";
import {PodLogsProps} from "./pod-logs-viewer";

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
                                 }: PodLogsProps & { fullscreen?: boolean }) => {
    const fullscreenURL =
        `/applications/${applicationNamespace}/${applicationName}/${namespace}/${containerName}/logs?` +
        `podName=${podName}&group=${group}&kind=${kind}&name=${name}`;
    return !fullscreen && (
        <Tooltip content='Fullscreen View'>
            <button className='argo-button argo-button--base'>
                <Link to={fullscreenURL} target='_blank'>
                    <i style={{color: '#fff'}} className='fa fa-external-link-alt'/>
                </Link>{' '}
            </button>
        </Tooltip>
    )
}