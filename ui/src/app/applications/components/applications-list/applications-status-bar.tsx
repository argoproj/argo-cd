import * as React from 'react';
import {COLORS, StatusBar, StatusBarReading} from '../../../shared/components';
import * as models from '../../../shared/models';
import {getAppSetHealthStatus} from '../utils';
import {AppsListStats} from './applications-list-stats';

const APP_HEALTH_READINGS: {name: models.HealthStatusCode; color: string}[] = [
    {name: 'Healthy', color: COLORS.health.healthy},
    {name: 'Progressing', color: COLORS.health.progressing},
    {name: 'Degraded', color: COLORS.health.degraded},
    {name: 'Suspended', color: COLORS.health.suspended},
    {name: 'Missing', color: COLORS.health.missing},
    {name: 'Unknown', color: COLORS.health.unknown}
];

function getAppReadings(stats: AppsListStats): StatusBarReading[] {
    return APP_HEALTH_READINGS.map(({name, color}) => ({name, value: stats.totalByHealthStatus[name] || 0, color}));
}

function getAppSetReadings(appSets: models.ApplicationSet[]): StatusBarReading[] {
    return [
        {
            name: 'Healthy',
            value: appSets.filter(appSet => getAppSetHealthStatus(appSet) === 'Healthy').length,
            color: COLORS.health.healthy
        },
        {
            name: 'Progressing',
            value: appSets.filter(appSet => getAppSetHealthStatus(appSet) === 'Progressing').length,
            color: COLORS.health.progressing
        },
        {
            name: 'Degraded',
            value: appSets.filter(appSet => getAppSetHealthStatus(appSet) === 'Degraded').length,
            color: COLORS.health.degraded
        },
        {
            name: 'Unknown',
            value: appSets.filter(appSet => getAppSetHealthStatus(appSet) === 'Unknown').length,
            color: COLORS.health.unknown
        }
    ];
}

export interface AppsStatusBarProps {
    stats: AppsListStats;
}

export const AppsStatusBar = ({stats}: AppsStatusBarProps) => {
    if (!stats || stats.total === 0) {
        return null;
    }
    return <StatusBar readings={getAppReadings(stats)} />;
};

export interface AppSetsStatusBarProps {
    appSets: models.ApplicationSet[];
}

export const AppSetsStatusBar = ({appSets}: AppSetsStatusBarProps) => {
    if (!appSets || appSets.length === 0) {
        return null;
    }
    return <StatusBar readings={getAppSetReadings(appSets)} />;
};
