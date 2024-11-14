import * as React from 'react';
import {createElement, useCallback} from 'react';
import {CaretUpOutlined, CaretDownOutlined, PlusCircleOutlined} from '@ant-design/icons';
import {Decoration, DecorationProps} from 'react-diff-view';

import './unfold.scss';

const ICON_TYPE_MAPPING = {
    up: CaretUpOutlined,
    down: CaretDownOutlined,
    none: PlusCircleOutlined
};

interface Props extends Omit<DecorationProps, 'children'> {
    start: number;
    end: number;
    direction: 'up' | 'down' | 'none';
    onExpand: (start: number, end: number) => void;
}

export default function Unfold({start, end, direction, onExpand, ...props}: Props) {
    const expand = useCallback(() => onExpand(start, end), [onExpand, start, end]);

    const iconType = ICON_TYPE_MAPPING[direction];
    const lines = end - start;

    return (
        <Decoration {...props}>
            <div className='unfold' onClick={expand}>
                {createElement(iconType)}
                &nbsp;Expand hidden {lines} lines
            </div>
        </Decoration>
    );
}
