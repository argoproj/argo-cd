import * as React from 'react';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import userEvent from '@testing-library/user-event';
import {ApplicationsQueryBar} from './applications-query-bar';
import {compileApplicationQuery} from './applications-query';
import {Application} from '../../../shared/models';

test('only submits valid queries and reports errors at their position', () => {
    const onSearch = jest.fn();
    render(<ApplicationsQueryBar value='' total={6} onSearch={onSearch} />);
    const input = screen.getByRole('combobox', {name: 'Search applications'});
    fireEvent.change(input, {target: {value: 'health ='}});
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByRole('alert')).toHaveTextContent('Enter a value');
    expect(screen.getByRole('button', {name: 'Search', exact: true})).toBeDisabled();
    fireEvent.submit(input.closest('form'));
    expect(onSearch).not.toHaveBeenCalled();
    fireEvent.change(input, {target: {value: 'health = Degraded OR sync = OutOfSync'}});
    expect(onSearch).not.toHaveBeenCalled();
    fireEvent.submit(input.closest('form'));
    expect(onSearch).toHaveBeenCalledWith('health = Degraded OR sync = OutOfSync');
});

test('restores the text from navigation and clears the applied query', () => {
    const onSearch = jest.fn();
    const {rerender} = render(<ApplicationsQueryBar value='health = Degraded' total={3} onSearch={onSearch} />);
    fireEvent.change(screen.getByRole('combobox'), {target: {value: 'draft'}});
    rerender(<ApplicationsQueryBar value='sync = OutOfSync' total={2} onSearch={onSearch} />);
    expect(screen.getByRole('combobox')).toHaveValue('sync = OutOfSync');
    fireEvent.click(screen.getByRole('button', {name: 'Clear'}));
    expect(onSearch).toHaveBeenCalledWith('');
    expect(screen.getByRole('combobox')).toHaveValue('');
});

test('copies the valid draft rather than the previously applied query', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined);
    const onSearch = jest.fn();
    Object.defineProperty(navigator, 'clipboard', {value: {writeText}, configurable: true});
    render(<ApplicationsQueryBar value='health = Healthy' total={2} onSearch={onSearch} />);
    fireEvent.change(screen.getByRole('combobox'), {target: {value: 'sync = OutOfSync OR health = Degraded'}});
    fireEvent.click(screen.getByRole('button', {name: 'Copy link'}));
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Link copied'));
    expect(new URL(writeText.mock.calls[0][0]).searchParams.get('q')).toBe('sync = OutOfSync OR health = Degraded');
    expect(onSearch).toHaveBeenCalledWith('sync = OutOfSync OR health = Degraded');
});

test('clipboard failure provides a usable fallback', async () => {
    Object.defineProperty(navigator, 'clipboard', {value: {writeText: jest.fn().mockRejectedValue(new Error('denied'))}, configurable: true});
    render(<ApplicationsQueryBar value='' total={6} onSearch={jest.fn()} />);
    fireEvent.click(screen.getByRole('button', {name: 'Copy link'}));
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('copy the browser address'));
});

test('completes fields, operators and values with the keyboard without applying prematurely', async () => {
    const user = userEvent.setup();
    const onSearch = jest.fn();
    render(<ApplicationsQueryBar value='' total={6} onSearch={onSearch} />);
    const input = screen.getByRole('combobox', {name: 'Search applications'});
    await user.type(input, 'sy');
    expect(screen.getByRole('option', {name: 'sync Field'})).toBeVisible();
    await user.tab();
    expect(input).toHaveValue('sync ');
    expect(input).toHaveFocus();
    await user.keyboard('{ArrowDown}{Enter}');
    expect(input).toHaveValue('sync = ');
    await user.type(input, 'out');
    await user.keyboard('{ArrowDown}{Enter}');
    expect(input).toHaveValue('sync = OutOfSync ');
    expect(onSearch).not.toHaveBeenCalled();
    await user.keyboard('{Enter}');
    expect(onSearch).toHaveBeenCalledWith('sync = OutOfSync');
});

test('supports implicit AND, lowercase connectors and mouse completion', async () => {
    const user = userEvent.setup();
    const onSearch = jest.fn();
    render(<ApplicationsQueryBar value='' total={6} onSearch={onSearch} />);
    const input = screen.getByRole('combobox');
    await user.type(input, 'auto');
    await user.click(screen.getByRole('option', {name: 'autoSync Field'}));
    await user.type(input, '= Disabled health != deg');
    await user.click(screen.getByRole('option', {name: 'Degraded health value'}));
    expect(input).toHaveValue('autoSync = Disabled health != Degraded ');
    await user.keyboard('{Enter}');
    expect(onSearch).toHaveBeenCalledWith('autoSync = Disabled health != Degraded');
    await user.type(input, ' or');
    expect(screen.getByRole('option', {name: 'OR Match either condition'})).toBeVisible();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
});

test('selects a multiline annotation suggestion without losing its line breaks in the input', async () => {
    const user = userEvent.setup();
    const note = 'first\nsecond';
    const onSearch = jest.fn();
    render(<ApplicationsQueryBar value='' total={1} onSearch={onSearch} values={{'annotation.note': [note]}} />);
    await user.type(screen.getByRole('combobox'), 'annotation.note = fi');
    await user.click(screen.getByRole('option'));
    expect(screen.getByRole('combobox')).toHaveValue(String.raw`annotation.note = "first\nsecond" `);
    await user.keyboard('{Enter}');
    const app = {metadata: {annotations: {note}}} as Application;
    expect(compileApplicationQuery(onSearch.mock.calls[0][0]).matches(app)).toBe(true);
});

test('completes at the caret and keeps the following conditions', async () => {
    const user = userEvent.setup();
    render(<ApplicationsQueryBar value='sync = OutOfSync OR health = Healthy' total={4} onSearch={jest.fn()} />);
    const input = screen.getByRole('combobox') as HTMLInputElement;
    await user.click(input);
    input.setSelectionRange(10, 10);
    fireEvent.select(input);
    await user.click(screen.getByRole('option', {name: 'OutOfSync sync value'}));
    expect(input).toHaveValue('sync = OutOfSync OR health = Healthy');
    expect(input.selectionStart).toBe(17);
});

test('Enter applies a valid query while continuation suggestions are open without a selection', async () => {
    const user = userEvent.setup();
    const onSearch = jest.fn();
    render(<ApplicationsQueryBar value='' total={6} onSearch={onSearch} />);
    const input = screen.getByRole('combobox');
    await user.type(input, '(sync = OutOfSync or health = Degraded)');
    expect(screen.getByRole('option', {name: 'AND Match both conditions'})).toBeVisible();
    await user.keyboard('{Enter}');
    expect(onSearch).toHaveBeenCalledWith('(sync = OutOfSync or health = Degraded)');
});

test('Tab with no suggestions and Shift Tab leave the input, and incomplete quotes can be completed', async () => {
    const user = userEvent.setup();
    render(<ApplicationsQueryBar value='' total={6} onSearch={jest.fn()} values={{project: ['Alice & Bob']}} />);
    const input = screen.getByRole('combobox');
    await user.type(input, 'project = "Al');
    await user.tab();
    expect(input).toHaveValue('project = "Alice & Bob" ');
    await user.tab();
    expect(screen.getByRole('button', {name: 'Search', exact: true})).toHaveFocus();
    await user.click(input);
    await user.tab({shift: true});
    expect(input).not.toHaveFocus();
    await user.clear(input);
    await user.type(input, 'name = nonexistent');
    await user.tab();
    expect(screen.getByRole('button', {name: 'Search', exact: true})).toHaveFocus();
});
