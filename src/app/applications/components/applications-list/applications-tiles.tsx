import { DropDownMenu} from 'argo-ui';
import * as moment from 'moment';
import * as React from 'react';

import { Consumer } from '../../../shared/context';
import * as models from '../../../shared/models';

import { ApplicationIngressLink } from '../application-ingress-link';
import * as AppUtils from '../utils';

// daysBeforeNow returns the delta, in days, between now and a given timestamp.
function daysBeforeNow(timestamp: string): number {
    const end = moment();
    const start = moment(timestamp);
    const delta = moment.duration(end.diff(start));
    return Math.round(delta.asDays());
}

export interface ApplicationTilesProps {
    applications: models.Application[];
    syncApplication: (appName: string, revision: string) => any;
    deleteApplication: (appName: string) => any;
}

export const ApplicationTiles = ({applications, syncApplication, deleteApplication}: ApplicationTilesProps) => (
    <Consumer>
    {(ctx) => (
    <div className='argo-table-list argo-table-list--clickable row small-up-1 medium-up-2 xxlarge-up-4'>
        {applications.map((app) => (
            <div key={app.metadata.name} className='column column-block'>
                <div className={`argo-table-list__row
                    applications-list__entry applications-list__entry--comparison-${app.status.sync.status}
                    applications-list__entry--health-${app.status.health.status}`
                }>
                    <div className='row' onClick={(e) => ctx.navigation.goto(`/applications/${app.metadata.name}`, {}, { event: e })}>
                        <div className='columns small-12 applications-list__info'>
                            <div className='applications-list__external-link'>
                                <ApplicationIngressLink ingress={app.status.ingress}/>
                            </div>
                            <div className='row'>
                                <div className='columns applications-list__title'>{app.metadata.name}</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Project:</div>
                                <div className='columns small-9'>{app.spec.project}</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Namespace:</div>
                                <div className='columns small-9'>{app.spec.destination.namespace}</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Cluster:</div>
                                <div className='columns small-9'>{app.spec.destination.server}</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Status:</div>
                                <div className='columns small-9'>
                                    <AppUtils.ComparisonStatusIcon status={app.status.sync.status}/> {app.status.sync.status}
                                </div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Health:</div>
                                <div className='columns small-9'>
                                    <AppUtils.HealthStatusIcon state={app.status.health}/> {app.status.health.status}
                                </div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Age:</div>
                                <div className='columns small-9'>{daysBeforeNow(app.metadata.creationTimestamp)} days</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Repository:</div>
                                <div className='columns small-9'>
                                    <a href={app.spec.source.repoURL} target='_blank' onClick={(event) => event.stopPropagation()}>
                                        <i className='fa fa-external-link'/> {app.spec.source.repoURL}
                                    </a>
                                </div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Path:</div>
                                <div className='columns small-9'>{app.spec.source.path}</div>
                            </div>
                            <div className='row'>
                                <div className='columns applications-list__entry--actions'>
                                    <DropDownMenu anchor={() =>
                                        <button className='argo-button argo-button--base-o'>Actions  <i className='fa fa-caret-down'/></button>
                                    } items={[
                                        { title: 'Sync', action: () => syncApplication(app.metadata.name, app.spec.source.targetRevision || 'HEAD') },
                                        { title: 'Delete', action: () => deleteApplication(app.metadata.name) },
                                    ]} />
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        ))}
    </div>
    )}
    </Consumer>
);
