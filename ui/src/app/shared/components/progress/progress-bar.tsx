import * as React from 'react';
import {COLORS} from '../colors';

export const ProgressBar = (props: {percentage: number}) => {
    return (
        <div
            style={{
                width: '90%',
                height: '10px',
                margin: '15px auto',
                background: COLORS.sync.unknown,
                borderRadius: '5px'
            }}>
            <div
                style={{
                    background: COLORS.sync.synced,
                    height: '100%',
                    width: Math.min(100, 100 * props.percentage) + '%',
                    borderRadius: 'inherit',
                    transition: 'width 0.1s ease-in'
                }}
            />
        </div>
    );
};
