import * as React from 'react';

export interface ArrowConnectorProps {
    color: string;
    left: number;
    top: number;
    angle: number;
}

export const ArrowConnector = (props: ArrowConnectorProps) => {
    const {color, left, top, angle} = props;
    return (
        <svg
            xmlns='http://www.w3.org/2000/svg'
            version='1.1'
            fill={color}
            width='11'
            height='11'
            viewBox='0 0 11 11'
            style={{left, top, transform: `translate(-10px, -10px) rotate(${angle}deg)`}}>
            <polygon points='11,5.5 0,11 0,0' />
        </svg>
    );
};
