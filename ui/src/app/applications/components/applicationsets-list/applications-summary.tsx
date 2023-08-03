import * as React from 'react';
const PieChart = require('react-svg-piechart').default;

import {COLORS} from '../../../shared/components';
import * as models from '../../../shared/models';
import {HealthStatusCode, ApplicationSetConditionType, SyncStatusCode, ApplicationSetStatus, ApplicationSetConditionStatus} from '../../../shared/models';
import {AppSetHealthStatusIcon, ComparisonStatusIcon, HealthStatusIcon} from './utils';

const healthColors = new Map<models.ApplicationSetConditionStatus, string>();
healthColors.set('Unknown', COLORS.health.unknown);
/*healthColors.set('Progressing', COLORS.health.progressing);
healthColors.set('Suspended', COLORS.health.suspended);
*/
healthColors.set('True', COLORS.health.healthy);
healthColors.set('False', COLORS.health.degraded);
// healthColors.set('Missing', COLORS.health.missing);

const syncColors = new Map<models.SyncStatusCode, string>();
syncColors.set('Unknown', COLORS.sync.unknown);
syncColors.set('Synced', COLORS.sync.synced);
syncColors.set('OutOfSync', COLORS.sync.out_of_sync);

export const ApplicationSetsSummary = ({applications}: {applications: models.ApplicationSet[]}) => {
 /*  const sync = new Map<string, number>();
    applications.forEach(app => sync.set(app.status.sync.status, (sync.get(app.status.sync.status) || 0) + 1));
    */
    const health = new Map<string, number>();
    applications.forEach(app => health.set(app.status.conditions[0].status, (health.get(app.status.conditions[0].status) || 0) + 1));

    const attributes = [
        {
            title: 'APPLICATIONSETS',
            value: applications.length
        },
    /*    {
            title: 'SYNCED',
            value: applications.filter(app => app.status.sync.status === 'Synced').length
        },
        */
        {
            title: 'HEALTHY',
            value: applications.filter(app => app.status.conditions[0].status === 'True').length
        },
        /*
        {
            title: 'CLUSTERS',
            value: new Set(applications.map(app => app.spec.destination.server)).size
        },
        
        {
            title: 'NAMESPACES',
            value: new Set(applications.map(app => app.spec.destination.namespace)).size
        }
        */
    ];

    const charts = [
      /*  {
            title: 'Sync',
            data: Array.from(sync.keys()).map(key => ({title: key, value: sync.get(key), color: syncColors.get(key as models.SyncStatusCode)})),
            legend: syncColors as Map<string, string>
        },
        */
        {
            title: 'Health',
            data: Array.from(health.keys()).map(key => ({title: key, value: health.get(key), color: healthColors.get(key as models.ApplicationSetConditionStatus)})),
            legend: healthColors as Map<string, string>
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
                                                             {/* {chart.title === 'Health' && <AppSetHealthStatusIcon state={{conditions : key as ApplicationSetConditionStatus}} noSpin={true} />}  */}
                                                            {chart.title === 'Sync' && <ComparisonStatusIcon status={key as SyncStatusCode} noSpin={true} />}
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
