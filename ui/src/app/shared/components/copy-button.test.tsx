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

    test('copies the text verbatim, preserving casing, and shows "Copied!" for 3 seconds', async () => {
        render(<CopyButton text='Guestbook-App' />);

        await act(async () => {
            fireEvent.click(screen.getByRole('button'));
        });
        expect(navigator.clipboard.writeText).toHaveBeenCalledWith('Guestbook-App');
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copied!');
        expect(screen.getByRole('status')).toHaveTextContent('Copied!');

        act(() => {
            jest.advanceTimersByTime(3000);
        });
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copy to clipboard');
        expect(screen.getByRole('status')).toHaveTextContent('');
    });

    test('falls back to execCommand when the clipboard API rejects', async () => {
        (navigator.clipboard.writeText as jest.Mock).mockRejectedValue(new Error('denied'));
        document.execCommand = jest.fn().mockReturnValue(true);
        render(<CopyButton text='guestbook' />);

        await act(async () => {
            fireEvent.click(screen.getByRole('button'));
        });
        expect(document.execCommand).toHaveBeenCalledWith('copy');
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copied!');
    });

    test('uses execCommand when the clipboard API is unavailable and reports failure if that fails too', async () => {
        Object.assign(navigator, {clipboard: undefined});
        document.execCommand = jest.fn().mockReturnValue(false);
        render(<CopyButton text='guestbook' />);

        await act(async () => {
            fireEvent.click(screen.getByRole('button'));
        });
        expect(document.execCommand).toHaveBeenCalledWith('copy');
        expect(screen.getByTestId('tooltip')).toHaveTextContent('Copy failed');
        expect(screen.getByRole('status')).toHaveTextContent('Copy failed');
    });

    test('re-announces repeated identical results', async () => {
        render(<CopyButton text='guestbook' />);
        const click = () =>
            act(async () => {
                fireEvent.click(screen.getByRole('button'));
            });

        await click();
        const first = screen.getByRole('status').firstElementChild;
        await click();

        expect(screen.getByRole('status')).toHaveTextContent('Copied!');
        expect(screen.getByRole('status').firstElementChild).not.toBe(first);
    });

    test('does not propagate the click to parents', async () => {
        const onParentClick = jest.fn();
        render(
            <div onClick={onParentClick}>
                <CopyButton text='guestbook' />
            </div>
        );

        await act(async () => {
            fireEvent.click(screen.getByRole('button'));
        });
        expect(onParentClick).not.toHaveBeenCalled();
    });

    test('renders nothing without text', () => {
        const {container} = render(<CopyButton text='' />);
        expect(container).toBeEmptyDOMElement();
    });

    test('does not start a reset timer if the copy resolves after unmount', async () => {
        const {unmount} = render(<CopyButton text='guestbook' />);
        const setTimeoutSpy = jest.spyOn(global, 'setTimeout');

        await act(async () => {
            fireEvent.click(screen.getByRole('button'));
            unmount();
        });

        expect(setTimeoutSpy).not.toHaveBeenCalledWith(expect.any(Function), 3000);
        setTimeoutSpy.mockRestore();
    });
});
