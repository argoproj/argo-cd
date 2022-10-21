import * as React from 'react';
import {CSSProperties, MouseEventHandler, ReactNode} from 'react';
import {Icon} from './icon';
import {Tooltip} from 'argo-ui';

export const Button = ({
    onClick,
    children,
    title,
    outline,
    icon,
    className,
    style,
    disabled
}: {
    onClick?: MouseEventHandler;
    children?: ReactNode;
    title?: string;
    outline?: boolean;
    icon?: Icon;
    className?: string;
    style?: CSSProperties;
    disabled?: boolean;
}) => (
    <Tooltip content={title}>
        <button
            className={'argo-button ' + (!outline ? 'argo-button--base' : 'argo-button--base-o') + ' ' + (disabled ? 'disabled' : '') + ' ' + (className || '')}
            style={style}
            onClick={onClick}>
            {icon && <i className={'fa fa-' + icon} />} {children}
        </button>
    </Tooltip>
);
