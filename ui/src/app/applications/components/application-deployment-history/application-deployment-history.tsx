import {DataLoader, DropDownMenu, Duration} from 'argo-ui';
import * as moment from 'moment';
import * as React from 'react';
import {Revision} from '../../../shared/components/revision';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {HealthStatusIcon} from '../utils';
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
                            <i className='fa fa-clock' /> <Timestamp date={info.deployedAt} />
                        </div>
                        <div>
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
                        <RevisionMetadataRows
                            applicationName={app.metadata.name}
                            source={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision}}
                        />
                        {info.status && (
                            <div className='row'>
                                <div className='columns small-3'>Status:</div>
                                <div className='columns small-9'>
                                    <HealthStatusIcon state={info.status.health} label={true} />
                                </div>
                            </div>
                        )}
                        <RevisionMetadataRows
                            applicationName={app.metadata.name}
                            source={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision}}
                        />
                        {selectedRollbackDeploymentIndex === index ? (
                            <DataLoader input={recentDeployments[index].source} load={src => services.repos.appDetails(src)}>
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
                        ) : null}
                    </div>
                </div>
            ))}
        </div>
    );
};
