import * as React from 'react';
import {render, fireEvent, cleanup} from '@testing-library/react';
import {Expandable} from './expandable';

// jsdom does not perform layout, so scrollHeight is always 0. Mock it to simulate
// content that either fits within, or overflows, the collapsed height.
function mockScrollHeight(value: number) {
    return jest.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(value);
}

afterEach(() => {
    jest.restoreAllMocks();
    cleanup();
});

describe('Expandable', () => {
    it('does not render the expand toggle when content fits within the collapsed height', () => {
        mockScrollHeight(20);
        const {container} = render(<Expandable height={48}>short annotation</Expandable>);

        expect(container.querySelector('.expandable__handle')).toBeNull();
        expect(container.querySelector('.expandable--collapsed')).toBeNull();
    });

    it('renders the expand toggle when content overflows the collapsed height', () => {
        mockScrollHeight(200);
        const {container} = render(<Expandable height={48}>a lot of content that overflows</Expandable>);

        const handle = container.querySelector('.expandable__handle');
        expect(handle).not.toBeNull();
        // collapsed by default -> chevron points down
        expect(handle?.classList.contains('fa-chevron-down')).toBe(true);
        expect(container.querySelector('.expandable--collapsed')).not.toBeNull();
    });

    it('toggles the expanded state when the handle is clicked', () => {
        mockScrollHeight(200);
        const {container} = render(<Expandable height={48}>a lot of content that overflows</Expandable>);

        const toggle = container.querySelector('.expandable a') as HTMLElement;
        fireEvent.click(toggle);

        expect(container.querySelector('.expandable__handle')?.classList.contains('fa-chevron-up')).toBe(true);
        expect(container.querySelector('.expandable--collapsed')).toBeNull();
    });

    it('detects overflow on mount even when ResizeObserver is unavailable', () => {
        // jsdom has no ResizeObserver; explicitly assert the non-RO fallback path so the
        // toggle is driven by the on-mount measurement rather than an observer callback.
        const original = (global as any).ResizeObserver;
        (global as any).ResizeObserver = undefined;
        try {
            mockScrollHeight(200);
            const {container} = render(<Expandable height={48}>a lot of content that overflows without a ResizeObserver</Expandable>);

            expect(container.querySelector('.expandable__handle')).not.toBeNull();
            expect(container.querySelector('.expandable--collapsed')).not.toBeNull();
        } finally {
            (global as any).ResizeObserver = original;
        }
    });

    it('keeps max-height numeric in both states so the CSS transition can animate', () => {
        mockScrollHeight(200);
        const {container} = render(<Expandable height={48}>a lot of content that overflows</Expandable>);

        const root = container.querySelector('.expandable') as HTMLElement;
        // Collapsed: max-height pinned to the collapsed height.
        expect(root.style.maxHeight).toBe('48px');

        fireEvent.click(container.querySelector('.expandable a') as HTMLElement);
        // Expanded: max-height is the measured content height (a concrete number), not
        // `none`/`auto`, so `transition: max-height` still animates.
        expect(root.style.maxHeight).toBe('200px');
    });

    it('does not constrain max-height when content fits within the collapsed height', () => {
        mockScrollHeight(20);
        const {container} = render(<Expandable height={48}>short annotation</Expandable>);

        const root = container.querySelector('.expandable') as HTMLElement;
        expect(root.style.maxHeight).toBe('');
    });
});
