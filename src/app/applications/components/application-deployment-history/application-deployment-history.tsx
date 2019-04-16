import { DataLoader, DropDownMenu, Duration } from 'argo-ui';
import * as moment from 'moment';
import * as React from 'react';

import * as models from '../../../shared/models';
import { services } from '../../../shared/services';
import { ApplicationParameters } from '../application-parameters/application-parameters';

require('./application-deployment-history.scss');

export const ApplicationDeploymentHistory = ({
    app,
    rollbackApp,
    selectedRollbackDeploymentIndex,
    selectDeployment,
}: {
    app: models.Application,
    selectedRollbackDeploymentIndex: number,
    rollbackApp: (info: models.RevisionHistory) => any,
    selectDeployment: (index: number) => any,
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
                        <i className='fa fa-clock'/>
                        <div>{info.deployedAt} - {info.nextDeployedAt || 'now'}</div>
                        <div><Duration durationMs={info.durationMs}/></div>
                    </div>
                    <div className='columns small-9'>
                        <div className='row'>
                            <div className='columns small-2'>
                                REVISION:
                            </div>
                            <div className='columns small-10'>
                                {info.revision}
                                <div className='application-deployment-history__item-menu'>
                                    <DropDownMenu anchor={() => <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                        <i className='fa fa-ellipsis-v'/>
                                    </button>} items={[{
                                        title: info.nextDeployedAt && 'Rollback' || 'Redeploy',
                                        action: () => rollbackApp(info),
                                    }]}/>
                                </div>
                            </div>
                        </div>
                        {selectedRollbackDeploymentIndex === index ? (
                            <DataLoader input={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision}}
                                load={(src) => services.repos.appDetails(src.repoURL, src.path, src.targetRevision, { helm: src.helm, ksonnet: src.ksonnet })}>
                            {(details: models.RepoAppDetails) =>
                                <ApplicationParameters application={{...app, spec: {...app.spec, source: recentDeployments[index].source} }} details={details} />
                            }
                            </DataLoader>
                        )
                        : null }
                    </div>
                </div>
            ))}
        </div>
    );
};
