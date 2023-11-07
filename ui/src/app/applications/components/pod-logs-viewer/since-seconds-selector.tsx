import * as React from 'react';
import {Tooltip} from 'argo-ui';

// SinceSelector is a component that renders a dropdown menu of time ranges
export const SinceSecondsSelector = ({sinceSeconds, setSinceSeconds}: {sinceSeconds: number; setSinceSeconds: (value: number) => void}) => (
    <Tooltip content='Show logs since a given time'>
        <select
            className='argo-field'
            style={{marginRight: '1em'}}
            value={sinceSeconds}
            onChange={e => {
                const v = parseInt(e.target.value, 10);
                setSinceSeconds(!isNaN(v) ? v : null);
            }}>
            <option value='60'>1m ago</option>
            <option value='300'>5m ago</option>
            <option value='600'>10m ago</option>
            <option value='1800'>30m ago</option>
            <option value='3600'>1h ago</option>
            <option value='14400'>4h ago</option>
            <option>forever</option>
        </select>
    </Tooltip>
);
