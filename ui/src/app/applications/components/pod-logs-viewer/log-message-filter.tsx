import * as React from 'react';
import {Tooltip} from 'argo-ui';
import {Spacer} from '../../../shared/components/spacer';

// Filter is a component that renders a filter input for log lines.
export const LogMessageFilter = ({filterText, setFilterText}: {filterText: string; setFilterText: (value: string) => void}) => (
    <>
        <label>containing</label>
        <Spacer />
        <Tooltip content='Filter log lines by text. Prefix with `!` to invert, e.g. `!foo` will find lines without `foo` in them'>
            <input className='argo-field' value={filterText} onChange={e => setFilterText(e.target.value)} />
        </Tooltip>
    </>
);
