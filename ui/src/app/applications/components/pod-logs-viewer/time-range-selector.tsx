import * as React from 'react';
import {Spacer} from '../../../shared/components/spacer';
import {Select} from 'argo-ui';

export type Option = 'min' | '1m' | '5m' | '30m' | '1h' | '4h';

export const TimeRangeSelector = ({since, setSince}: {since: Option; setSince: (value: Option) => void}) => (
    <>
        <label>from</label>
        <Spacer />
        <Select
            value={since}
            options={[
                {title: '1m ago', value: '1m'},
                {title: '5m ago', value: '5m'},
                {title: '30m ago', value: '30m'},
                {title: '1h ago', value: '1h'},
                {title: '4h ago', value: '4h'},
                {title: 'forever', value: 'min'}
            ]}
            onChange={option => {
                setSince(option.value as Option);
            }}
        />
    </>
);
