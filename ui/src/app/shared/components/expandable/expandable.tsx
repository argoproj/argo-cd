import classNames from 'classnames';
import * as React from 'react';

require('./expandable.scss');

export interface Props {
    height?: number;
    children?: React.ReactNode;
}

export const Expandable = (props: Props) => {
    const collapsedHeight = props.height || 100;
    const [expanded, setExpanded] = React.useState(false);
    const [contentHeight, setContentHeight] = React.useState(0);
    const contentEl = React.useRef<HTMLDivElement>(null);

    React.useLayoutEffect(() => {
        const measure = () => {
            if (contentEl.current) {
                // The inner element is never height-constrained, so scrollHeight is the
                // full natural height of the content regardless of the collapsed state.
                setContentHeight(contentEl.current.scrollHeight);
            }
        };
        measure();

        // Re-measure when the content resizes (e.g. async data loads or the window resizes)
        // so the toggle appears/disappears as the actual overflow changes.
        let observer: ResizeObserver | undefined;
        if (typeof ResizeObserver !== 'undefined' && contentEl.current) {
            observer = new ResizeObserver(() => measure());
            observer.observe(contentEl.current);
        }
        return () => observer?.disconnect();
    }, [props.children]);

    // Only offer the expand/collapse toggle when the content actually overflows the
    // collapsed height. A single short value (e.g. one short annotation) renders in full
    // with no near-invisible toggle. See https://github.com/argoproj/argo-cd/issues/29315.
    const overflowing = contentHeight > collapsedHeight;
    const collapsed = overflowing && !expanded;

    const style: React.CSSProperties = {};
    if (collapsed) {
        style.maxHeight = collapsedHeight;
    }

    return (
        <div style={style} className={classNames('expandable', {'expandable--collapsed': collapsed})}>
            <div ref={contentEl}>{props.children}</div>
            {overflowing && (
                <a onClick={() => setExpanded(!expanded)}>
                    <i className={classNames('expandable__handle fa', {'fa-chevron-down': !expanded, 'fa-chevron-up': expanded})} />
                </a>
            )}
        </div>
    );
};
