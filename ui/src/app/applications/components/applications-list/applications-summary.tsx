import * as React from 'react';
const PieChart = require('react-svg-piechart').default;

import {COLORS} from '../../../shared/components';
import * as models from '../../../shared/models';
import {HealthStatusCode, SyncStatusCode} from '../../../shared/models';
import {ComparisonStatusIcon, HealthStatusIcon, HydrateOperationPhaseIcon} from '../utils';

const healthColors = new Map<models.HealthStatusCode, string>();
healthColors.set('Unknown', COLORS.health.unknown);
healthColors.set('Progressing', COLORS.health.progressing);
healthColors.set('Suspended', COLORS.health.suspended);
healthColors.set('Healthy', COLORS.health.healthy);
healthColors.set('Degraded', COLORS.health.degraded);
healthColors.set('Missing', COLORS.health.missing);

const syncColors = new Map<models.SyncStatusCode, string>();
syncColors.set('Unknown', COLORS.sync.unknown);
syncColors.set('Synced', COLORS.sync.synced);
syncColors.set('OutOfSync', COLORS.sync.out_of_sync);

const hydratorColors = new Map<string, string>();
hydratorColors.set('Hydrating', COLORS.operation.running);
hydratorColors.set('Hydrated', COLORS.operation.success);
hydratorColors.set('Failed', COLORS.operation.failed);
hydratorColors.set('None', COLORS.sync.unknown);

export const ApplicationsSummary = ({applications, stats}: {applications: models.Application[]; stats: models.ApplicationListStats}) => {
    const sync = new Map<string, number>();
    if (stats.totalBySyncStatus) {
        Object.entries(stats.totalBySyncStatus).forEach(([key, value]) => sync.set(key, value));
    }

    const health = new Map<string, number>();
    if (stats.totalByHealthStatus) {
        Object.entries(stats.totalByHealthStatus).forEach(([key, value]) => health.set(key, value));
    }

    const hydrator = new Map<string, number>();
    applications.forEach(app => {
        const phase = app.status.sourceHydrator?.currentOperation?.phase || 'None';
        hydrator.set(phase, (hydrator.get(phase) || 0) + 1);
    });

    const clustersCount = new Set(stats.destinations.map(dest => dest.server || dest.name)).size;
    const namespacesCount = stats.namespaces.length;

    const attributes = [
        {
            title: 'APPLICATIONS',
            value: stats.total
        },
        {
            title: 'SYNCED',
            value: sync.get('Synced') || 0
        },
        {
            title: 'HEALTHY',
            value: health.get('Healthy') || 0
        },
        {
            title: 'HYDRATED',
            value: hydrator.get('Hydrated') || 0
        },
        {
            title: 'CLUSTERS',
            value: clustersCount
        },
        {
            title: 'NAMESPACES',
            value: namespacesCount
        }
    ];

    const charts = [
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
