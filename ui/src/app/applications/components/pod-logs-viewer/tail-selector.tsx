import * as React from 'react';
import {Tooltip} from 'argo-ui';

// TailSelector is a component that renders a dropdown menu of tail options
export const TailSelector = ({tail, setTail}: {tail: number; setTail: (value: number) => void}) => (
    <Tooltip content='Show the last N lines of the log'>
        <select className='argo-field' style={{marginRight: '1em'}} onChange={e => setTail(parseInt(e.target.value, 10))} value={tail.toString()}>
            <option value='100'>100 lines</option>
            <option value='500'>500 lines</option>
            <option value='1000'>1000 lines</option>
            <option value='5000'>5000 lines</option>
            <option value='10000'>10000 lines</option>
        </select>
    </Tooltip>
);
