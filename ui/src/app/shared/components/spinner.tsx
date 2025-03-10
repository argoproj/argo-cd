import * as React from 'react';
import {COLORS} from './colors';
import {SpinningIcon} from '../../applications/components/utils';

export const Spinner = ({show, style = {}}: {show: boolean; style?: React.CSSProperties}) =>
    show ? (
        <span style={style}>
            <SpinningIcon color={COLORS.operation.running} qeId='Spinnner-icon' />
        </span>
    ) : null;
