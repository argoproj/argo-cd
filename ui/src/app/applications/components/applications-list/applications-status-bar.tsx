import {Tooltip} from 'argo-ui/v2';
import * as React from 'react';
import {COLORS} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';

require('./applications-status-bar.scss');

export interface ApplicationStatusBarProps {
    applications: models.Application[];
}

export const ApplicationStatusBar = ({applications}: ApplicationStatusBarProps) => {
    const readings = [
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
            color: COLORS.sync.out_of_sync
        },
        {
            name: 'Missing, Unknown',
            value: applications.filter(app => app.status.health.status === 'Unknown' || app.status.health.status === 'Missing').length,
            color: COLORS.health.unknown
        }
    ];

    // will sort readings by value greatest to lowest, then by name
    readings.sort((a, b) => (a.value < b.value ? 1 : a.value === b.value ? (a.name > b.name ? 1 : -1) : -1));

    const totalItems = readings.reduce((total, i) => {
        return total + i.value;
    }, 0);

    const getTooltipContent = (item: {name: string; value: number; color: string}) => {
        if (item.name === 'Missing, Unknown') {
            const missing = applications.filter(app => app.status.health.status === 'Missing').length;
            const unknown = applications.filter(app => app.status.health.status === 'Unknown').length;
            if (missing) {
                if (unknown) {
                    return `${missing} Missing, ${unknown} Unknown`;
                }
                return `${missing} Missing`;
            }
            return `${unknown} Unknown`;
        } else {
            return item.value + ' ' + item.name;
        }
    };

    return (
        <Consumer>
            {ctx => (
                <div className='status-bar'>
                    <div className='scale'>
                        {readings &&
                            readings.length &&
                            readings.map((item, i) => {
                                if (item.value > 0) {
                                    return (
                                        <Tooltip content={getTooltipContent(item)} inverted={false} key={item.name}>
                                            <div className='segments' style={{backgroundColor: item.color, width: (item.value / totalItems) * 100 + '%'}} key={i} />
                                        </Tooltip>
                                    );
                                }
                            })}
                    </div>
                </div>
            )}
        </Consumer>
    );
};
