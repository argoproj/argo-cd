import * as React from 'react';
const PieChart = require('react-svg-piechart').default;

import {COLORS} from '../../../shared/components';
import * as models from '../../../shared/models';
import {HealthStatusCode, SyncStatusCode} from '../../../shared/models';
import {ComparisonStatusIcon, HealthStatusIcon, HydrateOperationPhaseIcon, getAppSetHealthStatus} from '../utils';

const healthColors = new Map<models.HealthStatusCode, string>();
healthColors.set('Unknown', COLORS.health.unknown);
healthColors.set('Progressing', COLORS.health.progressing);
healthColors.set('Suspended', COLORS.health.suspended);
healthColors.set('Healthy', COLORS.health.healthy);
healthColors.set('Degraded', COLORS.health.degraded);
healthColors.set('Missing', COLORS.health.missing);

const appSetHealthColors = new Map<models.HealthStatusCode, string>();
appSetHealthColors.set('Unknown', COLORS.health.unknown);
appSetHealthColors.set('Healthy', COLORS.health.healthy);
appSetHealthColors.set('Degraded', COLORS.health.degraded);
appSetHealthColors.set('Progressing', COLORS.health.progressing);

const syncColors = new Map<models.SyncStatusCode, string>();
syncColors.set('Unknown', COLORS.sync.unknown);
syncColors.set('Synced', COLORS.sync.synced);
syncColors.set('OutOfSync', COLORS.sync.out_of_sync);

const hydratorColors = new Map<string, string>();
hydratorColors.set('Hydrating', COLORS.operation.running);
hydratorColors.set('Hydrated', COLORS.operation.success);
hydratorColors.set('Failed', COLORS.operation.failed);
hydratorColors.set('None', COLORS.sync.unknown);

export const ApplicationsSummary = ({applications}: {applications: models.AbstractApplication[]}) => {
    const onAppsPath = !window.location.pathname.includes('applicationsets');

    const sync = new Map<string, number>();
    const health = new Map<string, number>();
    const hydrator = new Map<string, number>();

    if (onAppsPath) {
        const apps = applications as models.Application[];
        apps.forEach(app => sync.set(app.status.sync.status, (sync.get(app.status.sync.status) || 0) + 1));
        apps.forEach(app => health.set(app.status.health.status, (health.get(app.status.health.status) || 0) + 1));
        apps.forEach(app => {
            const phase = app.status.sourceHydrator?.currentOperation?.phase || 'None';
            hydrator.set(phase, (hydrator.get(phase) || 0) + 1);
        });
    } else {
        const appSets = applications as models.ApplicationSet[];
        appSets.forEach(appSet => {
            const status = getAppSetHealthStatus(appSet);
            health.set(status, (health.get(status) || 0) + 1);
        });
    }

    const attributes = onAppsPath
        ? [
              {title: 'APPLICATIONS', value: applications.length},
              {title: 'SYNCED', value: (applications as models.Application[]).filter(app => app.status.sync.status === 'Synced').length},
              {title: 'HEALTHY', value: (applications as models.Application[]).filter(app => app.status.health.status === 'Healthy').length},
              {title: 'HYDRATED', value: (applications as models.Application[]).filter(app => app.status.sourceHydrator?.currentOperation?.phase === 'Hydrated').length},
              {title: 'CLUSTERS', value: new Set((applications as models.Application[]).map(app => app.spec.destination.server || app.spec.destination.name)).size},
              {title: 'NAMESPACES', value: new Set((applications as models.Application[]).map(app => app.spec.destination.namespace)).size}
          ]
        : [
              {title: 'APPLICATIONSETS', value: applications.length},
              {title: 'HEALTHY', value: (applications as models.ApplicationSet[]).filter(app => getAppSetHealthStatus(app) === 'Healthy').length}
          ];

    const charts = onAppsPath
        ? [
              {
                  title: 'Sync',
                  data: Array.from(sync.keys()).map(key => ({title: key, value: sync.get(key), color: syncColors.get(key as models.SyncStatusCode)})),
                  legend: syncColors as Map<string, string>
              },
              {
                  title: 'Health',
                  data: Array.from(health.keys()).map(key => ({title: key, value: health.get(key), color: healthColors.get(key as models.HealthStatusCode)})),
                  legend: healthColors as Map<string, string>
              },
              {
                  title: 'Hydrator',
                  data: Array.from(hydrator.keys()).map(key => ({title: key, value: hydrator.get(key), color: hydratorColors.get(key)})),
                  legend: hydratorColors as Map<string, string>
              }
          ]
        : [
              {
                  title: 'Health',
                  data: Array.from(health.keys()).map(key => ({title: key, value: health.get(key), color: appSetHealthColors.get(key as models.HealthStatusCode)})),
                  legend: appSetHealthColors as Map<string, string>
              }
          ];

    return (
        <div className='white-box applications-list__summary'>
            <div className='row'>
                <div className='columns large-3 small-12'>
                    <div className='white-box__details'>
                        <p className='row'>SUMMARY</p>
                        {attributes.map(attr => (
                            <div className='row white-box__details-row' key={attr.title}>
                                <div className='columns small-8'>{attr.title}</div>
                                <div style={{textAlign: 'right'}} className='columns small-4'>
                                    {attr.value}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
                <div className='columns large-9 small-12'>
                    <div className='row chart-group'>
                        {charts.map(chart => {
                            const getLegendValue = (key: string) => {
                                const index = chart.data.findIndex((data: {title: string}) => data.title === key);
                                return index > -1 ? chart.data[index].value : 0;
                            };
                            return (
                                <React.Fragment key={chart.title}>
                                    <div className='columns large-6 small-12'>
                                        <div className='row chart'>
                                            <div className='large-8 small-6'>
                                                <h4 style={{textAlign: 'center'}}>{chart.title}</h4>
                                                <PieChart data={chart.data} />
                                            </div>
                                            <div className='large-3 small-1'>
                                                <ul>
                                                    {Array.from(chart.legend.keys()).map(key => (
                                                        <li style={{listStyle: 'none', whiteSpace: 'nowrap'}} key={key}>
                                                            {chart.title === 'Health' && <HealthStatusIcon state={{status: key as HealthStatusCode, message: ''}} noSpin={true} />}
                                                            {chart.title === 'Sync' && <ComparisonStatusIcon status={key as SyncStatusCode} noSpin={true} />}
                                                            {chart.title === 'Hydrator' && key !== 'None' && (
                                                                <HydrateOperationPhaseIcon
                                                                    operationState={{
                                                                        phase: key as any,
                                                                        startedAt: '',
                                                                        message: '',
                                                                        drySHA: '',
                                                                        hydratedSHA: '',
                                                                        sourceHydrator: {} as any
                                                                    }}
                                                                />
                                                            )}
                                                            {chart.title === 'Hydrator' && key === 'None' && (
                                                                <i className='fa fa-minus-circle' style={{color: hydratorColors.get(key)}} />
                                                            )}
                                                            {` ${key} (${getLegendValue(key)})`}
                                                        </li>
                                                    ))}
                                                </ul>
                                            </div>
                                        </div>
                                    </div>
                                </React.Fragment>
                            );
                        })}
                    </div>
                </div>
            </div>
        </div>
    );
};
