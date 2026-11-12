import * as React from 'react';
import {render, fireEvent, screen} from '@testing-library/react';

import {Filter} from './filter';

// avoids act() warnings from Tippy
jest.mock('argo-ui', () => ({
    Tooltip: (props: {children: React.ReactNode}) => <>{props.children}</>
}));

const Harness = (props: {field?: boolean; initial?: string[]; options: {label: string; count?: number}[]}) => {
    const [selected, setSelected] = React.useState<string[]>(props.initial || []);
    return (
        <div>
            <div data-testid='selected'>{JSON.stringify(selected)}</div>
            <button data-testid='clear-all' onClick={() => setSelected([])} />
            <Filter label='KINDS' selected={selected} setSelected={setSelected} options={props.options} field={!!props.field} negatable={true} />
        </div>
    );
};

const options = [{label: 'Pod'}, {label: 'Deployment'}];

const selected = () => JSON.parse(screen.getByTestId('selected').textContent);
const rows = () => Array.from(document.querySelectorAll('.filter__item'));
const includeButton = () => screen.queryByRole('button', {name: /Include/});
const excludeButton = () => screen.queryByRole('button', {name: /Exclude/});

const selectTag = (label: string) => {
    const input = screen.getByPlaceholderText('KINDS') as HTMLInputElement;
    fireEvent.focus(input);
    fireEvent.change(input, {target: {value: label}});
    const item = Array.from(document.querySelectorAll('.autocomplete__items__item')).find(el => el.textContent === label);
    fireEvent.click(item!);
};

test('the Include/Exclude switch only shows once a value is selected', () => {
    render(<Harness options={options} />);
    expect(excludeButton()).toBeNull();

    fireEvent.click(rows()[0]);
    expect(includeButton().getAttribute('aria-pressed')).toBe('true');
    expect(excludeButton().getAttribute('aria-pressed')).toBe('false');
});

test('Exclude prepends a single ! sentinel for the whole selection', () => {
    render(<Harness options={options} />);

    fireEvent.click(rows()[0]);
    fireEvent.click(rows()[1]);
    expect(selected()).toEqual(['Pod', 'Deployment']);

    fireEvent.click(excludeButton());
    expect(selected()).toEqual(['!', 'Pod', 'Deployment']);
    expect(excludeButton().getAttribute('aria-pressed')).toBe('true');
});

test('switching back to Include drops the sentinel', () => {
    render(<Harness options={options} />);

    fireEvent.click(rows()[0]);
    fireEvent.click(excludeButton());
    fireEvent.click(includeButton());
    expect(selected()).toEqual(['Pod']);
});

test('selected rows are marked as excluded only in Exclude mode', () => {
    render(<Harness options={options} />);

    fireEvent.click(rows()[0]);
    expect(document.querySelectorAll('.filter__item--excluded')).toHaveLength(0);

    fireEvent.click(excludeButton());
    const excluded = document.querySelectorAll('.filter__item--excluded');
    expect(excluded).toHaveLength(1);
    expect(excluded[0].textContent).toContain('Pod');
});

test('Exclude mode is restored from the sentinel, e.g. after a page reload', () => {
    render(<Harness initial={['!', 'Pod']} options={options} />);

    expect(excludeButton().getAttribute('aria-pressed')).toBe('true');
    expect(selected()).toEqual(['!', 'Pod']);
    expect(rows().filter(r => r.classList.contains('filter__item--selected'))).toHaveLength(1);
});

test('unchecking the last value falls back to Include, so the next selection is not silently excluded', () => {
    render(<Harness options={options} />);

    fireEvent.click(rows()[0]);
    fireEvent.click(excludeButton());
    fireEvent.click(rows()[0]);
    expect(selected()).toEqual([]);

    fireEvent.click(rows()[1]);
    expect(selected()).toEqual(['Deployment']);
});

test('CLEAR ALL from the parent also resets Exclude', () => {
    render(<Harness initial={['!', 'Pod']} options={options} />);

    fireEvent.click(screen.getByTestId('clear-all'));
    expect(selected()).toEqual([]);

    fireEvent.click(rows()[1]);
    expect(selected()).toEqual(['Deployment']);
});

test('field mode: a typed tag is excluded once Exclude is chosen', () => {
    render(<Harness field={true} options={options} />);

    selectTag('Pod');
    expect(selected()).toEqual(['Pod']);

    fireEvent.click(excludeButton());
    expect(selected()).toEqual(['!', 'Pod']);
    expect(rows()).toHaveLength(1);
});

test('field mode: a pre-selected tag picks up its count once options arrive later', () => {
    const {rerender} = render(<Filter label='KINDS' selected={['!', 'Pod']} setSelected={() => {}} options={[]} field={true} negatable={true} />);
    expect(rows()).toHaveLength(1);
    expect(rows()[0].textContent).not.toContain('10');

    rerender(<Filter label='KINDS' selected={['!', 'Pod']} setSelected={() => {}} options={[{label: 'Pod', count: 10}]} field={true} negatable={true} />);
    expect(rows()[0].textContent).toContain('10');
});
