import { DropDownMenu } from 'argo-ui';
import * as React from 'react';

import { Consumer } from '../../../shared/context';
import * as models from '../../../shared/models';
import { ApplicationURLs } from '../application-urls';
import * as AppUtils from '../utils';

export const ApplicationsTable = (props: {
    applications: models.Application[];
    syncApplication: (appName: string) => any;
    refreshApplication: (appName: string) => any;
    deleteApplication: (appName: string) => any;
}) => (
    <Consumer>
    {(ctx) => (
    <div className='argo-table-list argo-table-list--clickable'>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns large-3 small-6'>PROJECT/NAME</div>
                <div className='columns large-3 show-for-large'>SOURCE</div>
                <div className='columns large-1 small-2'>TARGET REVISION</div>
                <div className='columns large-3 show-for-large'>DESTINATION</div>
                <div className='columns large-2 small-4'>STATUS</div>
            </div>
        </div>
        {props.applications.map((app) => (
            <div key={app.metadata.name} className={`argo-table-list__row
                applications-list__entry applications-list__entry--comparison-${app.status.sync.status}
                applications-list__entry--health-${app.status.health.status}`
            }>
                <div className='row applications-list__table-row' onClick={(e) => ctx.navigation.goto(`/applications/${app.metadata.name}`, {}, { event: e })}>
                    <div className='columns large-3 small-6 wrap'>
                        <i className='icon argo-icon-application'/> {app.spec.project}/{app.metadata.name} <ApplicationURLs urls={app.status.summary.externalURLs}/>
                    </div>
                    <div className='columns large-3 show-for-large wrap'>
                        {app.spec.source.repoURL}/{app.spec.source.path}
                    </div>
                    <div className='columns large-1 small-2'>
                        {app.spec.source.targetRevision || 'HEAD'}
                    </div>
                    <div className='columns large-3 show-for-large wrap'>
                        {app.spec.destination.server}/{app.spec.destination.namespace}
                    </div>
                    <div className='columns large-2 small-4'>
                        <AppUtils.HealthStatusIcon state={app.status.health}/> <span>{app.status.health.status}</span>
                        &nbsp;
                        <AppUtils.ComparisonStatusIcon status={app.status.sync.status}/> <span>{app.status.sync.status}</span>
                        <DropDownMenu anchor={() => (
                            <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                <i className='fa fa-ellipsis-v'/>
                            </button>
                        )
                        } items={[
                            { title: 'Sync', action: () => props.syncApplication(app.metadata.name) },
                            { title: 'Refresh', action: () => props.refreshApplication(app.metadata.name) },
                            { title: 'Delete', action: () => props.deleteApplication(app.metadata.name) },
                        ]} />
                    </div>
                </div>
            </div>
        ))}
    </div>
    )}
    </Consumer>
);
