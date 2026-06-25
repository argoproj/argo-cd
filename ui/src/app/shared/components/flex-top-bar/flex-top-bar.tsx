import {SplitButton, Toolbar, Tooltip} from 'argo-ui';
import * as React from 'react';
import {Observable} from 'rxjs';
import {AuthOption, DataLoader} from '../';

import './flex-top-bar.scss';

type ActionMenuItem = NonNullable<Toolbar['actionMenu']>['items'][number];

function getTooltipContent(title: ActionMenuItem['title']): string | undefined {
    return typeof title === 'string' ? title : undefined;
}

function renderActionMenuLabel(title: ActionMenuItem['title']): string | React.ReactElement {
    if (typeof title === 'string') {
        return <span className='show-for-large'>{title}</span>;
    }
    return title;
}

const ActionButtonTooltip = (props: {content: string | undefined; needsAnchor?: boolean; children: React.ReactElement}) => {
    if (!props.content) {
        return props.children;
    }

    return (
        <Tooltip className='custom-tooltip' content={props.content}>
            {props.needsAnchor ? <span className='flex-top-bar__tooltip-anchor'>{props.children}</span> : props.children}
        </Tooltip>
    );
};

// Extend Toolbar type to support options on the right and auth control
export interface ToolbarWithOptions extends Toolbar {
    options?: React.ReactNode;
    addAuth?: boolean; // defaults to true
}

interface FlexTopBarProps {
    toolbar: ToolbarWithOptions | Observable<ToolbarWithOptions>;
}

export const FlexTopBar = (props: FlexTopBarProps) => {
    return (
        <React.Fragment>
            <div className='top-bar row flex-top-bar' key='tool-bar'>
                <DataLoader load={() => Promise.resolve(props.toolbar)}>{(toolbar: ToolbarWithOptions) => <FlexTopBarContent toolbar={toolbar} />}</DataLoader>
            </div>
            <div className='flex-top-bar__padder' />
        </React.Fragment>
    );
};

const FlexTopBarContent = (props: {toolbar: ToolbarWithOptions}) => {
    const shouldAddAuth = props.toolbar.addAuth !== false;

    return (
        <React.Fragment>
            <div className='flex-top-bar__actions'>
                {props.toolbar.actionMenu && (
                    <React.Fragment>
                        {props.toolbar.actionMenu.items.map((item, i) => {
                            const tooltipContent = getTooltipContent(item.title);
                            const key = item.qeId || i;

                            if (item.subActions && item.subActions.length > 0) {
                                return (
                                    <ActionButtonTooltip key={key} content={tooltipContent} needsAnchor={true}>
                                        <SplitButton
                                            action={item.action}
                                            title={renderActionMenuLabel(item.title)}
                                            iconClassName={item.iconClassName}
                                            subActions={item.subActions}
                                            disabled={item.disabled}
                                            qeId={item.qeId}
                                        />
                                    </ActionButtonTooltip>
                                );
                            }

                            return (
                                <ActionButtonTooltip key={key} content={tooltipContent}>
                                    <button
                                        disabled={!!item.disabled}
                                        qe-id={item.qeId}
                                        className='argo-button argo-button--base'
                                        onClick={() => item.action()}
                                        style={{marginRight: 2}}>
                                        {item.iconClassName && <i className={item.iconClassName} style={{marginLeft: '-5px', marginRight: '5px'}} />}
                                        {renderActionMenuLabel(item.title)}
                                    </button>
                                </ActionButtonTooltip>
                            );
                        })}
                    </React.Fragment>
                )}
            </div>
            <div className='flex-top-bar__tools'>
                {props.toolbar.tools && <div className='flex-top-bar__tools-left'>{props.toolbar.tools}</div>}
                {(props.toolbar.options || shouldAddAuth) && (
                    <div className='flex-top-bar__tools-right'>
                        {props.toolbar.options}
                        {shouldAddAuth && <AuthOption />}
                    </div>
                )}
            </div>
        </React.Fragment>
    );
};
