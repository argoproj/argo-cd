import {act, fireEvent, render, screen} from '@testing-library/react';
import * as React from 'react';

import {CopyButton} from './copy-button';

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

describe('CopyButton', () => {
    beforeEach(() => {
        jest.useFakeTimers();
        Object.assign(navigator, {clipboard: {writeText: jest.fn().mockResolvedValue(undefined)}});
    });

    afterEach(() => {
        jest.useRealTimers();
    });

    test('copies the text verbatim, preserving casing, and shows "Copied!" for 3 seconds', () => {
        render(<CopyButton text='Guestbook-App' />);

        fireEvent.click(screen.getByRole('button'));
        expect(navigator.clipboard.writeText).toHaveBeenCalledWith('Guestbook-App');
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copied!');

        act(() => {
            jest.advanceTimersByTime(3000);
        });
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copy to clipboard');
    });

    test('does not propagate the click to parents', () => {
        const onParentClick = jest.fn();
        render(
            <div onClick={onParentClick}>
                <CopyButton text='guestbook' />
            </div>
        );

        fireEvent.click(screen.getByRole('button'));
        expect(onParentClick).not.toHaveBeenCalled();
    });

    test('renders nothing without text and clears the timer on unmount', () => {
        const {container, unmount} = render(<CopyButton text='' />);
        expect(container).toBeEmptyDOMElement();
        unmount();

        const second = render(<CopyButton text='guestbook' />);
        fireEvent.click(screen.getByRole('button'));
        second.unmount();
        expect(jest.getTimerCount()).toBe(0);
    });
});
