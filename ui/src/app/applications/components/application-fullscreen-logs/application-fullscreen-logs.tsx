import * as React from 'react';
import Helmet from 'react-helmet';
import {RouteComponentProps} from 'react-router-dom';
import {PodLogsProps, PodsLogsViewer} from '../pod-logs-viewer/pod-logs-viewer';
import './application-fullscreen-logs.scss';

export const ApplicationFullscreenLogs = (props: RouteComponentProps<PodLogsProps>) => {
    const title = `${props.match.params.podName}/${props.match.params.containerName}`;
    return (
        <div className='application-fullscreen-logs'>
            <Helmet title={`${title} - Argo CD`} />
            <h4 style={{fontSize: '18px', textAlign: 'center'}}>{title}</h4>
            <PodsLogsViewer {...props.match.params} fullscreen={true} />
        </div>
    );
};
