import {DataLoader, DropDownMenu, Duration} from 'argo-ui';
import * as moment from 'moment';
import * as React from 'react';
import {Revision, Timestamp} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {RevisionMetadataRows} from './revision-metadata-rows';

require('./application-deployment-history.scss');

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
                            <i className='fa fa-clock' /> Deployed At:
                            <br />
                            <Timestamp date={info.deployedAt} />
                        </div>
                        <div>
                            <br />
                            <i className='fa fa-hourglass-half' /> Time to deploy:
                            <br />
                            {(info.deployStartedAt && <Duration durationMs={moment(info.deployedAt).diff(moment(info.deployStartedAt)) / 1000} />) || 'Unknown'}
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
                            <div className='columns small-3'>Revision:</div>
                            <div className='columns small-9'>
                                <Revision repoUrl={info.source.repoURL} revision={info.revision} />
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
                            <React.Fragment>
                                <RevisionMetadataRows
                                    applicationName={app.metadata.name}
                                    source={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision}}
                                />
                                <DataLoader
                                    input={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision, appName: app.metadata.name}}
                                    load={src => services.repos.appDetails(src, src.appName, app.spec.project)}>
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
                        ) : null}
                    </div>
                </div>
            ))}
        </div>
    );
};
