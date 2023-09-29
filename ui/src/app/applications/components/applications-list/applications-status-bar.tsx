import { Tooltip } from 'argo-ui/v2';
import * as React from 'react';
import { COLORS } from '../../../shared/components';
import { Consumer } from '../../../shared/context';
import * as models from '../../../shared/models';

import './applications-status-bar.scss';
import { getAppSetHealthStatus, isFromAppComponents } from '../utils';
import { Application, ApplicationSet } from '../../../shared/models';

export interface ApplicationsStatusBarProps {
    applications: models.AbstractApplication[];
}

export const ApplicationsStatusBar = ({ applications }: ApplicationsStatusBarProps) => {
    let readings: any[] = []
    if (isFromAppComponents()) {
        readings.push({
            name: 'Healthy',
            value: applications.filter(app => (app as Application).status.health.status === 'Healthy').length,
            color: COLORS.health.healthy
        },
            {
                name: 'Progressing',
                value: applications.filter(app => (app as Application).status.health.status === 'Progressing').length,
                color: COLORS.health.progressing
            },
            {
                name: 'Degraded',
                value: applications.filter(app => (app as Application).status.health.status === 'Degraded').length,
                color: COLORS.health.degraded
            },
            {
                name: 'Suspended',
                value: applications.filter(app => (app as Application).status.health.status === 'Suspended').length,
                color: COLORS.health.suspended
            },
            {
                name: 'Missing',
                value: applications.filter(app => (app as Application).status.health.status === 'Missing').length,
                color: COLORS.health.missing
            },
            {
                name: 'Unknown',
                value: applications.filter(app => (app as Application).status.health.status === 'Unknown').length,
                color: COLORS.health.unknown
            })
    }
    else {
        readings.push(
            {
                name: 'Healthy',
                value: applications.filter(app => getAppSetHealthStatus((app as ApplicationSet).status) === 'True').length,
                color: COLORS.health.healthy
            },
            {
                name: 'Degraded',
                value: applications.filter(app => getAppSetHealthStatus((app as ApplicationSet).status) === 'False').length,
                color: COLORS.health.degraded
            },
            {
                name: 'Unknown',
                value: applications.filter(app => getAppSetHealthStatus((app as ApplicationSet).status) === 'Unknown').length,
                color: COLORS.health.unknown
            }
        )
    }

    // will sort readings by value greatest to lowest, then by name
    readings.sort((a, b) => (a.value < b.value ? 1 : a.value === b.value ? (a.name > b.name ? 1 : -1) : -1));

    const totalItems = readings.reduce((total, i) => {
        return total + i.value;
    }, 0);

    return (
        <Consumer>
            {ctx => (
                <>
                    {totalItems > 1 && (
                        <div className='status-bar'>
                            {readings &&
                                readings.length > 1 &&
                                readings.map((item, i) => {
                                    if (item.value > 0) {
                                        return (
                                            <div className='status-bar__segment' style={{ backgroundColor: item.color, width: (item.value / totalItems) * 100 + '%' }} key={i}>
                                                <Tooltip content={`${item.value} ${item.name}`} inverted={true}>
                                                    <div className='status-bar__segment__fill' />
                                                </Tooltip>
                                            </div>
                                        );
                                    }
                                })}
                        </div>
                    )}
                </>
            )}
        </Consumer>
    );
};
