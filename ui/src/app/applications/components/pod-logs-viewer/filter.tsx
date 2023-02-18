import * as React from 'react';
import {Tooltip} from 'argo-ui';
import {Spacer} from '../../../shared/components/spacer';
import {Button} from '../../../shared/components/button';

export const Filter = ({filterText, setFilterText}: {filterText: string; setFilterText: (value: string) => void}) => (
    <>
        <label>containing</label>
        <Spacer />
        <Tooltip content='Filter log lines by text. Prefix with `!` to invert, e.g. `!foo` will find lines without `foo` in them'>
            <input placeholder='Filter' className='argo-field' value={filterText} onChange={e => setFilterText(e.target.value)} />
        </Tooltip>
        <Button onClick={() => setFilterText(filterText !== 'ERROR' ? 'ERROR' : '')} title='Errors'>
            <small>ERROR</small>
        </Button>
    </>
);
