import * as React from 'react';
import {COLORS, StatusBar, StatusBarReading} from '../../../shared/components';
import * as models from '../../../shared/models';
import {getAppSetHealthStatus} from '../utils';

function getAppReadings(applications: models.Application[]): StatusBarReading[] {
    return [
        {
            name: 'Healthy',
            value: applications.filter(app => app.status.health.status === 'Healthy').length,
            color: COLORS.health.healthy
        },
        {
            name: 'Progressing',
            value: applications.filter(app => app.status.health.status === 'Progressing').length,
            color: COLORS.health.progressing
        },
        {
            name: 'Degraded',
            value: applications.filter(app => app.status.health.status === 'Degraded').length,
            color: COLORS.health.degraded
        },
        {
            name: 'Suspended',
            value: applications.filter(app => app.status.health.status === 'Suspended').length,
            color: COLORS.health.suspended
        },
        {
            name: 'Missing',
            value: applications.filter(app => app.status.health.status === 'Missing').length,
            color: COLORS.health.missing
        },
        {
            name: 'Unknown',
            value: applications.filter(app => app.status.health.status === 'Unknown').length,
            color: COLORS.health.unknown
        }
    ];
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
    applications: models.Application[];
}

export const AppsStatusBar = ({applications}: AppsStatusBarProps) => {
    if (!applications || applications.length === 0) {
        return null;
    }
    return <StatusBar readings={getAppReadings(applications)} />;
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

// Legacy wrapper for backwards compatibility (callers should migrate to AppsStatusBar or AppSetsStatusBar)
export interface ApplicationsStatusBarProps {
    applications: models.Application[];
}

export const ApplicationsStatusBar = ({applications}: ApplicationsStatusBarProps) => {
    return <AppsStatusBar applications={applications} />;
};
