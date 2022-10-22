import {Select} from 'argo-ui';
import * as React from 'react';
import {Spacer} from '../../../shared/components/spacer';
import {Group} from '../../../shared/components/group';

const options = [
    {title: '10 lines', value: '10'},
    {title: '100 lines', value: '100'},
    {title: '500 lines', value: '500'}
];
export const TailSelector = ({tail, setTail}: {tail: number; setTail: (value: number) => void}) => (
    <Group>
        <label>tail</label>
        <Spacer />
        <Select value={tail.toString()} onChange={option => setTail(parseInt(option.value, 10))} options={options} />
    </Group>
);
