import * as React from 'react';

import Helmet from 'react-helmet';
import {RouteComponentProps} from 'react-router-dom';
import {Query} from '../../../shared/components';
import {Context} from '../../../shared/context';
import {PodsLogsViewer} from '../pod-logs-viewer/pod-logs-viewer';
import './application-fullscreen-logs.scss';

export const ApplicationFullscreenLogs = (props: RouteComponentProps<{name: string; container: string; namespace: string}>) => {
    const appContext = React.useContext(Context);
    return (
        <Query>
            {q => {
                const podName = q.get('podName');
                const name = q.get('name');
                const group = q.get('group');
                const kind = q.get('kind');
                const page = q.get('page');
                const untilTimes = (q.get('untilTimes') || '').split(',') || [];
                const title = `${podName || `${group}/${kind}/${name}`}:${props.match.params.container}`;
                return (
                    <div className='application-fullscreen-logs'>
                        <Helmet title={`${title} - Argo CD`} />
                        <h4 style={{fontSize: '18px', textAlign: 'center'}}>{title}</h4>
                        <PodsLogsViewer
                            applicationName={props.match.params.name}
                            containerName={props.match.params.container}
                            namespace={props.match.params.namespace}
                            group={group}
                            kind={kind}
                            name={name}
                            podName={podName}
                            fullscreen={true}
                            page={{number: parseInt(page, 10) || 0, untilTimes}}
                            setPage={pageData => appContext.navigation.goto('.', {page: pageData.number, untilTimes: pageData.untilTimes.join(',')})}
                        />
                    </div>
                );
            }}
        </Query>
    );
};
