import * as React from 'react';
import {MouseEventHandler, ReactNode} from 'react';
import {Icon} from './icon';
import {Tooltip} from 'argo-ui';

export const Button = ({
    onClick,
    children,
    title,
    outline,
    icon,
    className,
    disabled
}: {
    onClick?: MouseEventHandler;
    children?: ReactNode;
    title?: string;
    outline?: boolean;
    icon?: Icon;
    className?: string;
    disabled?: boolean;
}) => (
    <Tooltip content={title}>
        <button
            className={'argo-button ' + (!outline ? 'argo-button--base' : 'argo-button--base-o') + ' ' + (disabled ? 'disabled' : '') + ' ' + (className || '')}
            onClick={onClick}>
            {icon && <i className={'fa fa-' + icon} />} {children}
        </button>
    </Tooltip>
);
