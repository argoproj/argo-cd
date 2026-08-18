import {Tooltip} from 'argo-ui/v2';
import * as React from 'react';

import './status-bar.scss';

export interface StatusBarReading {
    name: string;
    value: number;
    color: string;
}

export interface StatusBarProps {
    readings: StatusBarReading[];
}

// StatusBar renders a horizontal proportional bar of colored segments (e.g. health breakdown of a
// list of applications/resources). Segments are sorted by value (desc) then name (asc), and the bar
// is only shown when there is more than one item to compare.
export const StatusBar = ({readings}: StatusBarProps) => {
    const sortedReadings = [...readings].sort((a, b) => (a.value < b.value ? 1 : a.value === b.value ? (a.name > b.name ? 1 : -1) : -1));

    const totalItems = sortedReadings.reduce((total, reading) => total + reading.value, 0);

    if (totalItems <= 1) {
        return null;
    }

    return (
        <div className='status-bar'>
            {sortedReadings.map((item, i) =>
                item.value > 0 ? (
                    <div className='status-bar__segment' style={{backgroundColor: item.color, width: (item.value / totalItems) * 100 + '%'}} key={i}>
                        <Tooltip content={`${item.value} ${item.name}`} inverted={true}>
                            <div className='status-bar__segment__fill' />
                        </Tooltip>
                    </div>
                ) : null
            )}
        </div>
    );
};
