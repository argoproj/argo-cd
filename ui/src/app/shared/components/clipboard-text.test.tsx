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

    test('renders custom children and label while copying original text casing', () => {
        render(
            <ClipboardText text='Guestbook-App' label='Copy application name'>
                GUESTBOOK-APP
            </ClipboardText>
        );

        expect(screen.getByText('GUESTBOOK-APP')).toBeTruthy();
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copy application name');
        fireEvent.click(screen.getByLabelText('Copy application name'));
        expect(navigator.clipboard.writeText).toHaveBeenCalledWith('Guestbook-App');
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copied!');
    });

    test('stops click propagation and falls back to execCommand when clipboard API is unavailable', () => {
        const onParentClick = jest.fn();
        Object.assign(navigator, {clipboard: undefined});
        document.execCommand = jest.fn().mockReturnValue(true);

        const {container} = render(
            <div onClick={onParentClick}>
                <ClipboardText text='guestbook' />
            </div>
        );

        clickCopy(container);
        expect(onParentClick).not.toHaveBeenCalled();
        expect(document.execCommand).toHaveBeenCalledWith('copy');
    });
});
