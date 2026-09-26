import {fireEvent, render, screen} from '@testing-library/react';
import * as React from 'react';

import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';
import {ApplicationsDetailsAppDropdown} from './application-details-app-dropdown';

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            list: jest.fn()
        }
    }
}));

const APPS = ['app-a', 'app-b', 'app-c'].map(name => ({kind: 'Application', metadata: {name, namespace: 'argocd'}}));

const listMock = services.applications.list as jest.Mock;
let goto: jest.Mock;
let scrolledInto: HTMLElement[];

beforeAll(() => {
    HTMLElement.prototype.scrollIntoView = function scrollIntoViewStub(this: HTMLElement) {
        scrolledInto.push(this);
    };
});

beforeEach(() => {
    // argo-ui's DropDown repositions itself on timers whose callback guards only on its own open
    // state, so a real timer firing after the test tree is torn down throws. Fake timers are only
    // advanced while the tree is mounted, which keeps that argo-ui behaviour out of these tests.
    jest.useFakeTimers();
    jest.clearAllMocks();
    listMock.mockResolvedValue({items: APPS});
    goto = jest.fn();
    scrolledInto = [];
});

afterEach(() => {
    jest.useRealTimers();
});

function renderDropdown(objectListKind = 'application') {
    render(
        <Context.Provider value={{navigation: {goto}} as any}>
            <ApplicationsDetailsAppDropdown appName='app-b' objectListKind={objectListKind} />
        </Context.Provider>
    );
    return screen.getByRole('button', {name: /app-b/});
}

async function openDropdown() {
    fireEvent.click(renderDropdown());
    await screen.findByRole('option', {name: /app-a/});
    return screen.getByRole('combobox');
}

function highlightedRow() {
    return screen.getAllByRole('option').find(option => option.getAttribute('aria-selected') === 'true');
}

function highlightedNames() {
    const row = highlightedRow();
    return row ? [row.textContent] : [];
}

test('arrow down walks the list, marking and scrolling to the highlighted row', async () => {
    const input = await openDropdown();
    expect(highlightedNames()).toEqual([]);

    fireEvent.keyDown(input, {key: 'ArrowDown'});
    expect(highlightedNames()).toEqual(['app-a']);
    expect(highlightedRow()).toHaveClass('application-details-app-dropdown__item--active');
    expect(scrolledInto[scrolledInto.length - 1]).toBe(highlightedRow());

    fireEvent.keyDown(input, {key: 'ArrowDown'});
    expect(highlightedNames()).toEqual(['app-b (current)']);
    expect(scrolledInto[scrolledInto.length - 1]).toBe(highlightedRow());

    fireEvent.keyDown(input, {key: 'ArrowUp'});
    expect(highlightedNames()).toEqual(['app-a']);
    expect(scrolledInto[scrolledInto.length - 1]).toBe(highlightedRow());
});

test('the highlight does not wrap around, and arrow up enters the list from the bottom', async () => {
    const input = await openDropdown();

    fireEvent.keyDown(input, {key: 'ArrowUp'});
    expect(highlightedNames()).toEqual(['app-c']);

    fireEvent.keyDown(input, {key: 'ArrowDown'});
    expect(highlightedNames()).toEqual(['app-c']);

    ['ArrowUp', 'ArrowUp', 'ArrowUp', 'ArrowUp'].forEach(key => fireEvent.keyDown(input, {key}));
    expect(highlightedNames()).toEqual(['app-a']);
});

test('the input exposes the highlighted row as the active descendant', async () => {
    const input = await openDropdown();
    expect(input).not.toHaveAttribute('aria-activedescendant');

    fireEvent.keyDown(input, {key: 'ArrowDown'});
    expect(input.getAttribute('aria-activedescendant')).toBe(highlightedRow().id);
});

test('the filter input takes focus when the menu opens', async () => {
    const input = await openDropdown();
    expect(input).toHaveFocus();
});

test('enter opens the highlighted application without reaching the argo-ui document handler', async () => {
    const documentKeydown = jest.fn();
    document.addEventListener('keydown', documentKeydown);
    try {
        const input = await openDropdown();

        fireEvent.keyDown(input, {key: 'ArrowDown'});
        fireEvent.keyDown(input, {key: 'ArrowDown'});
        fireEvent.keyDown(input, {key: 'Enter'});

        expect(goto).toHaveBeenCalledTimes(1);
        expect(goto).toHaveBeenCalledWith('/applications/argocd/app-b');
        // argo-ui's DropDown listens for Enter on document and clicks the menu's first <li>
        expect(documentKeydown).not.toHaveBeenCalled();
    } finally {
        document.removeEventListener('keydown', documentKeydown);
    }
});

test('typing highlights the top match, so enter always commits to a visible row', async () => {
    const input = await openDropdown();

    fireEvent.change(input, {target: {value: 'app-c'}});
    expect(screen.getAllByRole('option')).toHaveLength(1);
    expect(highlightedNames()).toEqual(['app-c']);

    fireEvent.keyDown(input, {key: 'Enter'});
    expect(goto).toHaveBeenCalledWith('/applications/argocd/app-c');
});

test('enter does nothing while no row is highlighted', async () => {
    const input = await openDropdown();

    fireEvent.keyDown(input, {key: 'Enter'});

    expect(goto).not.toHaveBeenCalled();
    expect(screen.getByRole('combobox')).toBeInTheDocument();
});

test('a filter that matches nothing leaves enter inert', async () => {
    const input = await openDropdown();

    fireEvent.change(input, {target: {value: 'nope'}});
    expect(screen.queryAllByRole('option')).toHaveLength(0);
    expect(screen.getByText('No matches')).toBeInTheDocument();

    fireEvent.keyDown(input, {key: 'ArrowDown'});
    fireEvent.keyDown(input, {key: 'Enter'});

    expect(goto).not.toHaveBeenCalled();
});

test('the mouse and the keyboard share one highlight', async () => {
    const input = await openDropdown();

    fireEvent.keyDown(input, {key: 'ArrowDown'});
    expect(highlightedNames()).toEqual(['app-a']);

    fireEvent.mouseMove(screen.getAllByRole('option')[2]);
    expect(highlightedNames()).toEqual(['app-c']);

    fireEvent.keyDown(input, {key: 'ArrowUp'});
    expect(highlightedNames()).toEqual(['app-b (current)']);
});

test('escape closes the menu and returns focus to the breadcrumb', async () => {
    const input = await openDropdown();

    fireEvent.keyDown(input, {key: 'Escape'});

    expect(screen.queryByRole('combobox')).toBeNull();
    expect(screen.getByRole('button', {name: /app-b/})).toHaveFocus();
});

test('the breadcrumb opens the menu from the keyboard', async () => {
    fireEvent.keyDown(renderDropdown(), {key: 'Enter'});

    expect(await screen.findByRole('combobox')).toBeInTheDocument();
});

test('reopening starts from an empty filter and refreshes the list', async () => {
    const input = await openDropdown();
    fireEvent.change(input, {target: {value: 'app-c'}});
    fireEvent.keyDown(input, {key: 'Escape'});

    fireEvent.click(screen.getByRole('button', {name: /app-b/}));

    expect(await screen.findByRole('combobox')).toHaveValue('');
    expect(screen.getAllByRole('option')).toHaveLength(3);
    expect(listMock).toHaveBeenCalledTimes(2);
});

test('a failed load is reported and retried on the next open', async () => {
    listMock.mockRejectedValueOnce(new Error('boom'));
    const anchor = renderDropdown();

    fireEvent.click(anchor);
    expect(await screen.findByText('Unable to load the list')).toBeInTheDocument();

    fireEvent.keyDown(screen.getByRole('combobox'), {key: 'Escape'});
    fireEvent.click(anchor);

    expect(await screen.findByRole('option', {name: /app-a/})).toBeInTheDocument();
    expect(screen.queryByText('Unable to load the list')).toBeNull();
});

test('application sets get their own labels and icon', async () => {
    fireEvent.click(renderDropdown('applicationset'));
    const options = await screen.findAllByRole('option');

    expect(screen.getByRole('listbox', {name: 'Application sets'})).toBeInTheDocument();
    expect(screen.getByRole('combobox', {name: 'Filter application sets'})).toBeInTheDocument();
    expect(options[0].querySelector('i')).toHaveClass('argo-icon-applicationset');
});
