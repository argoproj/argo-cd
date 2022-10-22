import * as React from 'react';
import {HelpIcon, Tooltip} from 'argo-ui';
import {Spacer} from '../../../shared/components/spacer';

export const Filter = ({filterText, setFilterText}: {filterText: string; setFilterText: (value: string) => void}) => (
    <>
        <label>containing</label>
        <Spacer />
        <Tooltip content='Filter log lines by text'>
            <input placeholder='Filter' className='argo-field' value={filterText} onChange={e => setFilterText(e.target.value)} />
        </Tooltip>
        <HelpIcon title='Prefix with `!` to invert, e.g. `!error` will find lines without `error` in them' />
    </>
);
