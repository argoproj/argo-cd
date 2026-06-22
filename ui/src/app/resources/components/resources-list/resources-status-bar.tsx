import * as React from 'react';
import {COLORS, StatusBar, StatusBarReading} from '../../../shared/components';
import * as models from '../../../shared/models';
import {resourceHealthStatus} from '../utils';

export interface ResourcesStatusBarProps {
    resources: models.Resource[];
}

export const ResourcesStatusBar = ({resources}: ResourcesStatusBarProps) => {
    const readings: StatusBarReading[] = [
        {
            name: 'Healthy',
            value: resources.filter(resource => resourceHealthStatus(resource) === 'Healthy').length,
            color: COLORS.health.healthy
        },
        {
            name: 'Progressing',
            value: resources.filter(resource => resourceHealthStatus(resource) === 'Progressing').length,
            color: COLORS.health.progressing
        },
        {
            name: 'Degraded',
            value: resources.filter(resource => resourceHealthStatus(resource) === 'Degraded').length,
            color: COLORS.health.degraded
        },
        {
            name: 'Suspended',
            value: resources.filter(resource => resourceHealthStatus(resource) === 'Suspended').length,
            color: COLORS.health.suspended
        },
        {
            name: 'Missing',
            value: resources.filter(resource => resourceHealthStatus(resource) === 'Missing').length,
            color: COLORS.health.missing
        },
        {
            name: 'Unknown',
            value: resources.filter(resource => resourceHealthStatus(resource) === 'Unknown').length,
            color: COLORS.health.unknown
        }
    ];

    return <StatusBar readings={readings} />;
};
