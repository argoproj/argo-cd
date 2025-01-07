import * as moment from 'moment';
import * as React from 'react';
import * as models from '../../../shared/models';
import './application-deployment-history.scss';
import {DataLoader} from 'argo-ui';
import {Revision} from '../../../shared/components';
import {services} from '../../../shared/services';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {RevisionMetadataRows} from './revision-metadata-rows';

type props = {
    app: models.Application;
    info: models.RevisionHistory;
    index: number;
};

export const ApplicationDeploymentHistoryDetails = ({app, info, index}: props) => {
    const deployments = (app.status.history || []).slice().reverse();
    const recentDeployments = deployments.map((info, i) => {
        const nextDeployedAt = i === 0 ? null : deployments[i - 1].deployedAt;
        const runEnd = nextDeployedAt ? moment(nextDeployedAt) : moment();
        return {...info, nextDeployedAt, durationMs: runEnd.diff(moment(info.deployedAt)) / 1000};
    });

    const [showParameterDetails, setShowParameterDetails] = React.useState(Boolean);

    return (
        <>
            {info.sources === undefined ? (
                <React.Fragment>
                    <div>
                        <div className='row'>
                            <div className='columns small-3'>Revision:</div>
                            <div className='columns small-9'>
                                <Revision repoUrl={info.source.repoURL} revision={info.revision} />
                            </div>
                        </div>
                    </div>
                    <RevisionMetadataRows
                        applicationName={app.metadata.name}
                        applicationNamespace={app.metadata.namespace}
                        source={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision}}
                        index={0}
                        versionId={recentDeployments[index].id}
                    />
                    <button
                        type='button'
                        className='argo-button argo-button--base application-deployment-history__show-parameter-details'
                        onClick={() => setShowParameterDetails(!showParameterDetails)}>
                        {showParameterDetails ? 'Hide details' : 'Show details'}
                    </button>

                    {showParameterDetails && (
                        <DataLoader
                            input={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision, appName: app.metadata.name}}
                            load={src => services.repos.appDetails(src, src.appName, app.spec.project, 0, recentDeployments[index].id)}>
                            {(details: models.RepoAppDetails) => (
                                <div>
                                    <ApplicationParameters
                                        application={{
                                            ...app,
                                            spec: {...app.spec, source: recentDeployments[index].source}
                                        }}
                                        details={details}
                                    />
                                </div>
                            )}
                        </DataLoader>
                    )}
                </React.Fragment>
            ) : (
                info.sources.map((source, i) => (
                    <React.Fragment key={`${index}_${i}`}>
                        {i > 0 ? <div className='separator' /> : null}
                        <div>
                            <div className='row'>
                                <div className='columns small-3'>Revision:</div>
                                <div className='columns small-9'>
                                    <Revision repoUrl={source.repoURL} revision={info.revisions[i]} />
                                </div>
                            </div>
                        </div>
                        <RevisionMetadataRows
                            applicationName={app.metadata.name}
                            applicationNamespace={app.metadata.namespace}
                            source={{...source, targetRevision: recentDeployments[index].revisions[i]}}
                            index={i}
                            versionId={recentDeployments[index].id}
                        />
                        <button
                            type='button'
                            className='argo-button argo-button--base application-deployment-history__show-parameter-details'
                            onClick={() => setShowParameterDetails(!showParameterDetails)}>
                            {showParameterDetails ? 'Hide details' : 'Show details'}
                        </button>

                        {showParameterDetails && (
                            <DataLoader
                                input={{
                                    ...source,
                                    targetRevision: recentDeployments[index].revisions[i],
                                    index: i,
                                    versionId: recentDeployments[index].id,
                                    appName: app.metadata.name
                                }}
                                load={src => services.repos.appDetails(src, src.appName, app.spec.project, i, recentDeployments[index].id)}>
                                {(details: models.RepoAppDetails) => (
                                    <div>
                                        <ApplicationParameters
                                            application={{
                                                ...app,
                                                spec: {...app.spec, source}
                                            }}
                                            details={details}
                                        />
                                    </div>
                                )}
                            </DataLoader>
                        )}
                    </React.Fragment>
                ))
            )}
        </>
    );
};
