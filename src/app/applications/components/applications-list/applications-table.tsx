import * as React from 'react';

import { Consumer } from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';

export const ApplicationsTable = (props: { applications: models.Application[] }) => (
    <Consumer>
    {(ctx) => (
    <div className='argo-table-list argo-table-list--clickable'>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns small-2'>PROJECT/NAME</div>
                <div className='columns small-2'>STATUS</div>
                <div className='columns small-4'>SOURCE</div>
                <div className='columns small-4'>DESTINATION</div>
            </div>
        </div>
        {props.applications.map((app) => (
            <div key={app.metadata.name} className={`argo-table-list__row
                applications-list__entry applications-list__entry--comparison-${app.status.sync.status}
                applications-list__entry--health-${app.status.health.status}`
            }>
                <div className='row applications-list__table-row' onClick={(e) => ctx.navigation.goto(`/applications/${app.metadata.name}`, {}, e)}>
                    <div className='columns small-2'>
                        <i className='icon argo-icon-application'/> {app.spec.project}/{app.metadata.name}
                    </div>
                    <div className='columns small-1'>
                        <AppUtils.ComparisonStatusIcon status={app.status.sync.status}/> {app.status.sync.status}
                    </div>
                    <div className='columns small-1'>
                        <AppUtils.HealthStatusIcon state={app.status.health}/> {app.status.health.status}
                    </div>
                    <div className='columns small-4'>
                        {app.spec.source.repoURL}/{app.spec.source.path}
                    </div>
                    <div className='columns small-4'>
                        {app.spec.destination.server}/{app.spec.destination.namespace}
                    </div>
                </div>
            </div>
        ))}
    </div>
    )}
    </Consumer>
);
