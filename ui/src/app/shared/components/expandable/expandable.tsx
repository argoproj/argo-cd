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
        const cleanups: Array<() => void> = [];
        if (typeof ResizeObserver !== 'undefined' && contentEl.current) {
            const observer = new ResizeObserver(() => measure());
            observer.observe(contentEl.current);
            cleanups.push(() => observer.disconnect());
        } else {
            // Fallback for environments without ResizeObserver: watch DOM mutations
            // (async content changes) and window resizes (wrap-driven height changes)
            // so overflow detection stays accurate beyond the initial measurement.
            if (typeof MutationObserver !== 'undefined' && contentEl.current) {
                const mutationObserver = new MutationObserver(() => measure());
                mutationObserver.observe(contentEl.current, {childList: true, subtree: true, characterData: true});
                cleanups.push(() => mutationObserver.disconnect());
            }
            if (typeof window !== 'undefined') {
                window.addEventListener('resize', measure);
                cleanups.push(() => window.removeEventListener('resize', measure));
            }
        }
        return () => cleanups.forEach(cleanup => cleanup());
    }, [props.children]);

    // Only offer the expand/collapse toggle when the content actually overflows the
    // collapsed height. A single short value (e.g. one short annotation) renders in full
    // with no near-invisible toggle. See https://github.com/argoproj/argo-cd/issues/29315.
    const overflowing = contentHeight > collapsedHeight;
    const collapsed = overflowing && !expanded;

    // Drive the `transition: max-height` in expandable.scss by keeping max-height a
    // concrete number in both states while overflowing: collapse to the fixed height,
    // expand to the measured content height. CSS cannot animate to/from `none`, so the
    // expanded state must use an explicit height rather than leaving max-height unset.
    const style: React.CSSProperties = {};
    if (overflowing) {
        style.maxHeight = collapsed ? collapsedHeight : contentHeight;
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
