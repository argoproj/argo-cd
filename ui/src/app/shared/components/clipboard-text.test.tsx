import {act, fireEvent, render, screen} from '@testing-library/react';
import * as React from 'react';

import {ClipboardText} from './clipboard-text';

// Render the tooltip content inline so the test can read it without hovering.
jest.mock('argo-ui', () => ({
    ...jest.requireActual('argo-ui'),
    Tooltip: ({content, children}: {content: React.ReactNode; children: React.ReactNode}) => (
        <>
            {children}
            <span data-testid='tooltip'>{content}</span>
        </>
    )
}));

describe('ClipboardText', () => {
    beforeEach(() => {
        jest.useFakeTimers();
        Object.assign(navigator, {clipboard: {writeText: jest.fn().mockResolvedValue(undefined)}});
    });

    afterEach(() => {
        jest.useRealTimers();
    });

    const clickCopy = (container: HTMLElement) => fireEvent.click(container.querySelector('.fa-clipboard'));

    test('shows "Copied!" for 3 seconds after the last click', () => {
        const {container} = render(<ClipboardText text='guestbook' />);

        clickCopy(container);
        expect(navigator.clipboard.writeText).toHaveBeenCalledWith('guestbook');
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copied!');

        act(() => {
            jest.advanceTimersByTime(2000);
        });
        clickCopy(container);

        act(() => {
            jest.advanceTimersByTime(2000);
        });
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copied!');

        act(() => {
            jest.advanceTimersByTime(1000);
        });
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copy to clipboard');
        expect(jest.getTimerCount()).toBe(0);
    });

    test('clears the pending timer on unmount', () => {
        const {container, unmount} = render(<ClipboardText text='guestbook' />);

        clickCopy(container);
        unmount();

        expect(jest.getTimerCount()).toBe(0);
    });
});
