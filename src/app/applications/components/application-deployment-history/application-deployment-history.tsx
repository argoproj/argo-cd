import { DropDownMenu, Duration } from 'argo-ui';
import * as moment from 'moment';
import * as React from 'react';

import * as models from '../../../shared/models';

import { DataLoader } from '../../../shared/components';
import { services } from '../../../shared/services';
import { getParamsWithOverridesInfo } from '../utils';

require('./application-deployment-history.scss');

export const ApplicationDeploymentHistory = ({
    app,
    selectedRollbackDeploymentIndex,
    rollbackApp,
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
                        <i className='icon argo-icon-clock'/>
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
                            <DataLoader load={() => services.applications.getManifest(app.metadata.name, info.revision)}>
                                {(manifest) => {
                                    const componentParams = getParamsWithOverridesInfo(manifest.params, info.componentParameterOverrides || []);
                                    return Array.from(componentParams.keys()).map((component: string) => (
                                        componentParams.get(component).map((param: (models.ComponentParameter & {original: string})) => (
                                            <div className='row' key={param.component + param.name}>
                                            <div className='columns small-4 application-deployment-history__param-name'>
                                            <span>{param.component}.{param.name}:</span>
                                            </div>
                                            <div className='columns small-8'>
                                                <span title={param.value}>
                                                    {param.original && <span className='fa fa-exclamation-triangle' title={`Original value: ${param.original}`}/>}
                                                    {param.value}
                                                </span>
                                            </div>
                                        </div>
                                        ))
                                    ));
                                }}
                            </DataLoader>
                        )
                        : null }
                    </div>
                </div>
            ))}
        </div>
    );
};
