import * as classNames from 'classnames';
import * as React from 'react';

require('./expandable.scss');

export interface Props extends React.Props<any> {
    height?: number;
}

export const Expandable = (props: Props) => {
    const [expanded, setExpanded] = React.useState(false);
    const contentEl = React.useRef(null);
    const style: React.CSSProperties = {};
    if (!expanded) {
        style.maxHeight = props.height || 100;
    } else {
        style.maxHeight = (contentEl.current && contentEl.current.clientHeight) || 10000;
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
