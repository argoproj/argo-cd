import * as React from 'react';
import {Tooltip} from 'argo-ui';

// Filter is a component that renders a filter input for log lines.
export const LogMessageFilter = ({filterText, setFilterText}: {filterText: string; setFilterText: (value: string) => void}) => (
    <Tooltip content='Filter log lines by text. Prefix with `!` to invert, e.g. `!foo` will find lines without `foo` in them'>
        <input className='argo-field' placeholder='containing' value={filterText} onChange={e => setFilterText(e.target.value)} />
    </Tooltip>
);
