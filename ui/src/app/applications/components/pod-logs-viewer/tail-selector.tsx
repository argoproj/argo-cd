import {Select} from 'argo-ui';
import * as React from 'react';
import {Spacer} from '../../../shared/components/spacer';

const options = [
    {title: '100 lines', value: '100'},
    {title: '1000 lines', value: '1000'},
    {title: '5000 lines', value: '5000'}
];
export const TailSelector = ({tail, setTail}: {tail: number; setTail: (value: number) => void}) => (
    <>
        <label>tail</label>
        <Spacer />
        <Select value={tail.toString()} onChange={option => setTail(parseInt(option.value, 10))} options={options} />
    </>
);
