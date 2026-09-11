import classNames from 'classnames';
import * as React from 'react';

require('./expandable.scss');

export interface Props {
    height?: number;
    children?: React.ReactNode;
}

export const Expandable = (props: Props) => {
    const [expanded, setExpanded] = React.useState(false);
    const [contentHeight, setContentHeight] = React.useState(0);
    const contentEl = React.useRef<HTMLDivElement>(null);

    React.useLayoutEffect(() => {
        if (expanded && contentEl.current) {
            setContentHeight(contentEl.current.clientHeight);
        }
    }, [expanded]);

    const style: React.CSSProperties = {};
    if (!expanded) {
        style.maxHeight = props.height || 100;
    } else {
        style.maxHeight = contentHeight || 10000;
    }

    return (
        <div style={style} className={classNames('expandable', {'expandable--collapsed': !expanded})}>
            <div ref={contentEl}>{props.children}</div>
            <a onClick={() => setExpanded(!expanded)}>
                <i className={classNames('expandable__handle fa', {'fa-chevron-down': !expanded, 'fa-chevron-up': expanded})} />
            </a>
        </div>
    );
};
