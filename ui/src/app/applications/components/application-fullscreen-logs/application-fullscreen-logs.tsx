import {PodLogsProps, PodsLogsViewer} from '../pod-logs-viewer/pod-logs-viewer';
import * as React from 'react';
import {RouteComponentProps} from 'react-router-dom';

import './application-fullscreen-logs.scss';

export const ApplicationFullscreenLogs = (props: RouteComponentProps<PodLogsProps>) => {
    return (
        <div className='application-fullscreen-logs'>
            <PodsLogsViewer {...props.match.params} />
        </div>
    );
};
