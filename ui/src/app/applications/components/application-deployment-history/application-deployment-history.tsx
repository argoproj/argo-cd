import {DataLoader, DropDownMenu, Duration} from 'argo-ui';
import {InitiatedBy} from './initiated-by';
import * as moment from 'moment';
import * as React from 'react';
import {Revision, Timestamp} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {RevisionMetadataRows} from './revision-metadata-rows';
import './application-deployment-history.scss';

export const ApplicationDeploymentHistory = ({
    app,
    rollbackApp,
    selectedRollbackDeploymentIndex,
    selectDeployment
}: {
    app: models.Application;
    selectedRollbackDeploymentIndex: number;
    rollbackApp: (info: models.RevisionHistory) => any;
    selectDeployment: (index: number) => any;
}) => {
    const deployments = (app.status.history || []).slice().reverse();
    const recentDeployments = deployments.map((info, i) => {
        const nextDeployedAt = i === 0 ? null : deployments[i - 1].deployedAt;
        const runEnd = nextDeployedAt ? moment(nextDeployedAt) : moment();
        return {...info, nextDeployedAt, durationMs: runEnd.diff(moment(info.deployedAt)) / 1000};
    });
    return (
        <div className='application-deployment-history'>
            {recentDeployments.map((info, index) => (
                <div className='row application-deployment-history__item' key={info.deployedAt} onClick={() => selectDeployment(index)}>
                    <div className='columns small-3'>
                        <div>
                            <i className='fa fa-clock' /> <span className='show-for-large'>Deployed At:</span>
                            <br />
                            <Timestamp date={info.deployedAt} />
                        </div>
                        <div>
                            <br />
                            <i className='fa fa-hourglass-half' /> <span className='show-for-large'>Time to deploy:</span>
                            <br />
                            {(info.deployStartedAt && <Duration durationMs={moment(info.deployedAt).diff(moment(info.deployStartedAt)) / 1000} />) || 'Unknown'}
                        </div>
                        <div>
                            <br />
                            Initiated by:
                            <br />
                            <InitiatedBy username={info.initiatedBy.username} automated={info.initiatedBy.automated} />
                        </div>
                        <div>
                            <br />
                            Active for:
                            <br />
                            <Duration durationMs={info.durationMs} />
                        </div>
                    </div>
                    <div className='columns small-9'>
                        <div className='row'>
                            <div className='columns small-9'>
                                <div className='application-deployment-history__item-menu'>
                                    <DropDownMenu
                                        anchor={() => (
                                            <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                <i className='fa fa-ellipsis-v' />
                                            </button>
                                        )}
                                        items={[
                                            {
                                                title: (info.nextDeployedAt && 'Rollback') || 'Redeploy',
                                                action: () => rollbackApp(info)
                                            }
                                        ]}
                                    />
                                </div>
                            </div>
                        </div>
                        {selectedRollbackDeploymentIndex === index ? (
                            info.sources === undefined ? (
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
                                    </React.Fragment>
                                ))
                            )
                        ) : (
                            <p>Click to see source details.</p>
                        )}
                    </div>
                </div>
            ))}
        </div>
    );
};
