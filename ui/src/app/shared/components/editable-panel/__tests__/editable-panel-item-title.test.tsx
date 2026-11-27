import * as React from 'react';
import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {EditablePanelItemTitle} from '../editable-panel';

const hint = 'Use $values/path/to/values.yaml to read values from another Git source.';

function renderHint(onSubmit = jest.fn()) {
    render(
        <form onSubmit={onSubmit}>
            <EditablePanelItemTitle item={{title: 'VALUES FILES', hint, view: 'No values files selected'}} />
            <button type='button'>Next field</button>
        </form>
    );
}

test('opens the hint on keyboard focus and dismisses it with Escape without moving focus', async () => {
    const user = userEvent.setup();
    renderHint();
    const help = screen.getByRole('button', {name: 'Help for VALUES FILES'});

    await user.tab();

    expect(help).toHaveFocus();
    expect(await screen.findByRole('tooltip')).toBeVisible();
    await waitFor(() => expect(help).toHaveAccessibleDescription(hint));

    await user.keyboard('{Escape}');

    await waitFor(() => expect(screen.queryByRole('tooltip')).not.toBeInTheDocument());
    expect(help).toHaveFocus();

    await user.tab();
    await user.tab({shift: true});

    expect(help).toHaveFocus();
    expect(await screen.findByRole('tooltip')).toBeVisible();
    await user.tab();
    await waitFor(() => expect(screen.queryByRole('tooltip')).not.toBeInTheDocument());
});

test('keeps the hint available on mouse hover after dismissal', async () => {
    const user = userEvent.setup();
    renderHint();
    const help = screen.getByRole('button', {name: 'Help for VALUES FILES'});

    await user.tab();
    await user.keyboard('{Escape}');
    await user.tab();
    await user.hover(help);

    expect(await screen.findByRole('tooltip')).toBeVisible();
});

test('does not submit its containing form when help is activated', async () => {
    const user = userEvent.setup();
    const onSubmit = jest.fn(event => event.preventDefault());
    renderHint(onSubmit);
    const help = screen.getByRole('button', {name: 'Help for VALUES FILES'});

    await user.tab();
    await user.keyboard('{Enter}');
    await user.click(help);

    expect(onSubmit).not.toHaveBeenCalled();
});
