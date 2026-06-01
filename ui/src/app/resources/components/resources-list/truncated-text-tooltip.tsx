import {Tooltip} from 'argo-ui';
import * as React from 'react';

export function useTruncatedElement<T extends HTMLElement>(text: string) {
    const ref = React.useRef<T>(null);
    const [isTruncated, setIsTruncated] = React.useState(false);

    const update = React.useCallback(() => {
        const el = ref.current;
        if (!el) {
            setIsTruncated(false);
            return;
        }
        setIsTruncated(el.scrollWidth > el.clientWidth);
    }, []);

    React.useLayoutEffect(() => {
        update();
    }, [text, update]);

    React.useEffect(() => {
        const el = ref.current;
        if (!el || typeof ResizeObserver === 'undefined') {
            return;
        }
        const observer = new ResizeObserver(() => update());
        observer.observe(el);
        return () => observer.disconnect();
    }, [text, update]);

    return {ref, isTruncated};
}

export const TruncatedTextTooltip = (props: {content: string; className?: string; children?: React.ReactNode}) => {
    const text = props.content ?? '';
    const {ref, isTruncated} = useTruncatedElement<HTMLSpanElement>(text);

    return (
        <Tooltip content={text} enabled={!!text && isTruncated}>
            <span ref={ref} className={props.className}>
                {props.children ?? text}
            </span>
        </Tooltip>
    );
};
