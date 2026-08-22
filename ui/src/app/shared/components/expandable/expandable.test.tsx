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
});
