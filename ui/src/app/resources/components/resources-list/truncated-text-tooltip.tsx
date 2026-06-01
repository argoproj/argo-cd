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

let legendMeasureContext: CanvasRenderingContext2D | null = null;

function measureLegendTextWidth(element: HTMLElement, text: string): number {
    if (!legendMeasureContext) {
        legendMeasureContext = document.createElement('canvas').getContext('2d');
    }
    if (!legendMeasureContext) {
        return 0;
    }
    const style = window.getComputedStyle(element);
    legendMeasureContext.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
    return legendMeasureContext.measureText(text).width;
}

/** Truncates label with "…" when needed; suffix (e.g. " (24)") is always kept visible. */
export function useLegendDisplayText(label: string, value: number) {
    const ref = React.useRef<HTMLSpanElement>(null);
    const suffix = ` (${value})`;
    const fullText = `${label}${suffix}`;
    const [displayText, setDisplayText] = React.useState(fullText);

    const update = React.useCallback(() => {
        const el = ref.current;
        const parent = el?.parentElement;
        if (!el || !parent) {
            return;
        }

        const availableWidth = parent.getBoundingClientRect().width;
        if (availableWidth <= 0) {
            return;
        }

        const fits = (text: string) => measureLegendTextWidth(el, text) <= availableWidth;

        if (fits(fullText)) {
            setDisplayText(fullText);
            return;
        }

        let lo = 0;
        let hi = label.length;
        let best = 0;
        while (lo <= hi) {
            const mid = Math.floor((lo + hi) / 2);
            const truncatedLabel = mid >= label.length ? label : `${label.slice(0, mid)}…`;
            const text = `${truncatedLabel}${suffix}`;
            if (fits(text)) {
                best = mid;
                lo = mid + 1;
            } else {
                hi = mid - 1;
            }
        }

        const truncatedLabel = best >= label.length ? label : `${label.slice(0, best)}…`;
        setDisplayText(`${truncatedLabel}${suffix}`);
    }, [label, suffix, fullText]);

    React.useLayoutEffect(() => {
        update();
    }, [update]);

    React.useEffect(() => {
        const el = ref.current;
        const target = el?.parentElement;
        if (!el || !target || typeof ResizeObserver === 'undefined') {
            return;
        }
        const observer = new ResizeObserver(() => update());
        observer.observe(target);
        return () => observer.disconnect();
    }, [update]);

    return {ref, displayText, isTruncated: displayText !== fullText};
}

export const TruncatedTextTooltip = (props: {content: string; tooltipContent?: string; className?: string; children?: React.ReactNode}) => {
    const text = props.content ?? '';
    const tooltipText = props.tooltipContent ?? text;
    const {ref, isTruncated} = useTruncatedElement<HTMLSpanElement>(text);

    return (
        <Tooltip content={tooltipText} enabled={!!tooltipText && isTruncated}>
            <span ref={ref} className={props.className}>
                {props.children ?? text}
            </span>
        </Tooltip>
    );
};
