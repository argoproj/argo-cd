import * as React from 'react';

import Helmet from 'react-helmet';
import {RouteComponentProps} from 'react-router-dom';
import {Spinner} from '../../../shared/components';
import {ErrorBoundary} from '../../../shared/components/error-boundary/error-boundary';
import {useQuery} from '../../../shared/hooks/query';
import './application-fullscreen-logs.scss';

const PodsLogsViewer = React.lazy(() => import(/* webpackChunkName: "pod-logs" */ '../pod-logs-viewer/pod-logs-viewer').then(m => ({default: m.PodsLogsViewer})));

export const ApplicationFullscreenLogs = (props: RouteComponentProps<{name: string; appnamespace: string; container: string; namespace: string}>) => {
    const query = useQuery();

    const podName = query.get('podName');
    const name = query.get('name');
    const group = query.get('group');
    const kind = query.get('kind');
    const title = `${podName || `${group}/${kind}/${name}`}:${props.match.params.container}`;
    const fullscreen = true;
    return (
        <div className='application-fullscreen-logs'>
            <Helmet title={`${title} - Argo CD`} />
            <h4 style={{fontSize: '18px', textAlign: 'center'}}>{title}</h4>
            <ErrorBoundary message='Failed to load logs viewer. Please reload and try again.'>
                <React.Suspense fallback={<Spinner show={true} />}>
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
                </React.Suspense>
            </ErrorBoundary>
        </div>
    );
};
