import * as React from 'react';

import Helmet from 'react-helmet';
import {RouteComponentProps} from 'react-router-dom';
import {Query} from '../../../shared/components';
import {PodsLogsViewer} from '../pod-logs-viewer/pod-logs-viewer';
import './application-fullscreen-logs.scss';

export const ApplicationFullscreenLogs = (props: RouteComponentProps<{name: string; appnamespace: string; container: string; namespace: string}>) => {
    return (
        <Query>
            {q => {
                const podName = q.get('podName');
                const name = q.get('name');
                const group = q.get('group');
                const kind = q.get('kind');
                const title = `${podName || `${group}/${kind}/${name}`}:${props.match.params.container}`;
                const fullscreen = true;
                return (
                    <div className='application-fullscreen-logs'>
                        <Helmet title={`${title} - Argo CD`} />
                        <h4 style={{fontSize: '18px', textAlign: 'center'}}>{title}</h4>
                        <PodsLogsViewer
                            applicationName={props.match.params.name}
                            applicationNamespace={props.match.params.appnamespace}
                            containerName={props.match.params.container}
                            namespace={props.match.params.namespace}
                            group={group}
                            kind={kind}
                            name={name}
                            podName={podName}
                            fullscreen={fullscreen}
                        />
                    </div>
                );
            }}
        </Query>
    );
};
