import * as React from 'react';
import {Tooltip} from 'argo-ui';

// TailSelector is a component that renders a dropdown menu of tail options
export const TailSelector = ({tail, setTail}: {tail: number; setTail: (value: number) => void}) => (
    <Tooltip content='Show the last N lines of the log'>
        <select className='argo-field' onChange={e => setTail(parseInt(e.target.value, 10))} value={tail.toString()}>
            {[100, 1000, 5000].map(n => (
                <option key={n} value={n}>
                    {n}
                </option>
            ))}
        </select>
    </Tooltip>
);
