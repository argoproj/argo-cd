import * as React from 'react';
import {render} from '@testing-library/react';

import {CheckboxRow} from './filter';

const tooltipProps: Array<{placement?: string}> = [];

jest.mock('argo-ui', () => ({
    Tooltip: (props: {placement?: string; children: React.ReactNode}) => {
        tooltipProps.push(props);
        return <>{props.children}</>;
    }
}));

jest.mock('argo-ui/v2', () => ({
    Autocomplete: () => null,
    Checkbox: () => <input type='checkbox' />
}));

describe('CheckboxRow', () => {
    beforeEach(() => {
        tooltipProps.length = 0;
    });

    // A tooltip placed above the row covers the options listed above it (and the autocomplete input),
    // which makes it hard to select several values in a row.
    test('shows the label tooltip beside the row so it does not cover the rows above', () => {
        render(<CheckboxRow value={false} option={{label: 'production-eu-west-1', count: 2}} />);

        expect(tooltipProps).toHaveLength(1);
        expect(tooltipProps[0].placement).toBe('right');
    });
});
