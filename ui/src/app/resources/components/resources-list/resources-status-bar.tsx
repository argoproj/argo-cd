import {Tooltip} from 'argo-ui/v2';
import * as React from 'react';
import {COLORS} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';

import './resources-status-bar.scss';

export interface ResourcesStatusBarProps {
    resources: models.Resource[];
}

export const ResourcesStatusBar = ({resources}: ResourcesStatusBarProps) => {
    const readings = [
        {
            name: 'Healthy',
            value: resources.filter(resource => resource?.health?.status === 'Healthy').length,
            color: COLORS.health.healthy
        },
        {
            name: 'Progressing',
            value: resources.filter(resource => resource?.health?.status === 'Progressing').length,
            color: COLORS.health.progressing
        },
        {
            name: 'Degraded',
            value: resources.filter(resource => resource?.health?.status === 'Degraded').length,
            color: COLORS.health.degraded
        },
        {
            name: 'Suspended',
            value: resources.filter(resource => resource?.health?.status === 'Suspended').length,
            color: COLORS.health.suspended
        },
        {
            name: 'Missing',
            value: resources.filter(resource => resource?.health?.status === 'Missing').length,
            color: COLORS.health.missing
        },
        {
            name: 'Unknown',
            value: resources.filter(resource => resource?.health?.status === 'Unknown' || !resource?.health).length,
            color: COLORS.health.unknown
        }
    ];

    // will sort readings by value greatest to lowest, then by name
    readings.sort((a, b) => (a.value < b.value ? 1 : a.value === b.value ? (a.name > b.name ? 1 : -1) : -1));

    const totalItems = readings.reduce((total, i) => {
        return total + i.value;
    }, 0);

    return (
        <Consumer>
            {() => (
                <>
                    {totalItems > 1 && (
                        <div className='status-bar'>
                            {readings &&
                                readings.length > 1 &&
                                readings.map((item, i) => {
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
};
