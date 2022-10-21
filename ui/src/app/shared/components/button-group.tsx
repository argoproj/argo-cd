import {ReactNode} from 'react';
import * as React from 'react';

export const ButtonGroup = ({children}: {children: ReactNode}) => (
    <span
        style={{
            paddingRight: '20px'
        }}>
        {children}
    </span>
);
