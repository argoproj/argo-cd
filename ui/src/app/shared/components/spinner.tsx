import * as React from 'react';
import {COLORS} from './colors';
import {FontAwesomeIcon} from '@fortawesome/react-fontawesome';
import {faCircleNotch} from '@fortawesome/free-solid-svg-icons';

export const Spinner = ({show, style = {}}: {show: boolean; style?: React.CSSProperties}) =>
    show ? (
        <span style={style}>
            <FontAwesomeIcon icon={faCircleNotch} spin style={{color: COLORS.operation.running}} title='Loading' />
        </span>
    ) : null;
