import {DropDownMenu, Duration} from 'argo-ui';
import {InitiatedBy} from './initiated-by';
import * as moment from 'moment';
import * as React from 'react';
import {Timestamp} from '../../../shared/components';
import * as models from '../../../shared/models';
import './application-deployment-history.scss';
import {ApplicationDeploymentHistoryDetails} from './application-deployment-history-details';

export const ApplicationDeploymentHistory = ({
    app,
    rollbackApp,
    selectDeployment
}: {
    app: models.Application;
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

                        <ApplicationDeploymentHistoryDetails index={index} info={info} app={app} />
                    </div>
                </div>
            ))}
        </div>
    );
};
