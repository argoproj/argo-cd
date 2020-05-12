import * as React from 'react';
import {COLORS} from './colors';

export const Spinner = ({show, style = {}}: {show: boolean; style?: React.CSSProperties}) =>
    show ? (
        <span style={style}>
            <i className='fa fa-circle-notch fa-spin' style={{color: COLORS.operation.running}} />
        </span>
    ) : null;
