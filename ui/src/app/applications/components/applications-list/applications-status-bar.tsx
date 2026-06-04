import {Tooltip} from 'argo-ui/v2';
import * as React from 'react';
import {COLORS} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {getAppSetHealthStatus} from '../utils';

import './applications-status-bar.scss';

interface Reading {
    name: string;
    value: number;
    color: string;
}

function getAppReadings(applications: models.Application[]): Reading[] {
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

function getAppSetReadings(appSets: models.ApplicationSet[]): Reading[] {
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

function StatusBarRenderer({readings}: {readings: Reading[]}) {
    // will sort readings by value greatest to lowest, then by name
    const sortedReadings = [...readings].sort((a, b) => (a.value < b.value ? 1 : a.value === b.value ? (a.name > b.name ? 1 : -1) : -1));

    const totalItems = sortedReadings.reduce((total, i) => {
        return total + i.value;
    }, 0);

    return (
        <Consumer>
            {() => (
                <>
                    {totalItems > 1 && (
                        <div className='status-bar'>
                            {sortedReadings &&
                                sortedReadings.length > 1 &&
                                sortedReadings.map((item, i) => {
                                    if (item.value > 0) {
                                        return (
                                            <div className='status-bar__segment' style={{backgroundColor: item.color, width: (item.value / totalItems) * 100 + '%'}} key={i}>
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
}

export interface AppsStatusBarProps {
    applications: models.Application[];
}

export const AppsStatusBar = ({applications}: AppsStatusBarProps) => {
    if (!applications || applications.length === 0) {
        return null;
    }
    return <StatusBarRenderer readings={getAppReadings(applications)} />;
};

export interface AppSetsStatusBarProps {
    appSets: models.ApplicationSet[];
}

export const AppSetsStatusBar = ({appSets}: AppSetsStatusBarProps) => {
    if (!appSets || appSets.length === 0) {
        return null;
    }
    return <StatusBarRenderer readings={getAppSetReadings(appSets)} />;
};

// Legacy wrapper for backwards compatibility (callers should migrate to AppsStatusBar or AppSetsStatusBar)
export interface ApplicationsStatusBarProps {
    applications: models.Application[];
}

export const ApplicationsStatusBar = ({applications}: ApplicationsStatusBarProps) => {
    return <AppsStatusBar applications={applications} />;
};
