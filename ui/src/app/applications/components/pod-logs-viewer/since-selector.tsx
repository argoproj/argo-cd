import * as React from 'react';
import {Since} from '../../../shared/services/since';
import {Tooltip} from 'argo-ui';

// SinceSelector is a component that renders a dropdown menu of time ranges
export const SinceSelector = ({since, setSince}: {since: Since; setSince: (value: Since) => void}) => (
    <Tooltip content='Show logs since a given time'>
        <select className='argo-field' value={since} onChange={e => setSince(e.target.value as Since)}>
            <option value='1m ago'>1m ago</option>
            <option value='5m ago'>5m ago</option>
            <option value='30m ago'>30m ago</option>
            <option value='1h ago'>1h ago</option>
            <option value='4h ago'>4h ago</option>
            <option value='forever'>forever</option>
        </select>
    </Tooltip>
);
