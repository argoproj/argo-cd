import * as React from 'react';
import {Spacer} from '../../../shared/components/spacer';
import {Select} from 'argo-ui';
import {Since} from '../../../shared/services/since';

// TimeRangeSelector is a component that renders a dropdown menu of time ranges
export const SinceSelector = ({since, setSince}: {since: Since; setSince: (value: Since) => void}) => (
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
                setSince(option.value as Since);
            }}
        />
    </>
);
