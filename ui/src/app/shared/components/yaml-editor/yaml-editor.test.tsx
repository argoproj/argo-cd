import * as React from 'react';
import {act} from 'react';
import {render as rtlRender, screen, fireEvent} from '@testing-library/react';
import * as jsYaml from 'js-yaml';

import {Context} from '../../context';
import {dumpYaml} from './yaml-merge-patch';

let lastMonacoText = '';
const fakeModel = {
    lines: '',
    setValue: jest.fn((value: string) => {
        fakeModel.lines = value;
    }),
    getFullModelRange: jest.fn(() => ({startLineNumber: 1, startColumn: 1, endLineNumber: 99, endColumn: 1})),
    pushEditOperations: jest.fn((_before: unknown, edits: {text: string}[]) => {
        fakeModel.lines = edits[0].text;
        return null;
    }),
    getLinesContent: jest.fn(() => fakeModel.lines.split('\n'))
};

jest.mock('../monaco-editor', () => ({
    MonacoEditor: (props: {editor?: {input: {text: string}; getApi?: (api: any) => void}}) => {
        lastMonacoText = props.editor?.input.text ?? '';
        props.editor?.getApi?.({getModel: () => fakeModel});
        return React.createElement('div', {'data-testid': 'monaco'});
    }
}));

import {YamlEditor} from './yaml-editor';

const mockContext = {
    history: {} as any,
    popup: {} as any,
    navigation: {} as any,
    baseHref: '/',
    notifications: {show: jest.fn()} as any
};

function renderEditor(element: React.ReactElement) {
    return rtlRender(React.createElement(Context.Provider, {value: mockContext}, element));
}

const liveA = {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: {name: 'guestbook-ui', resourceVersion: '100', annotations: {tick: '1'}},
    spec: {replicas: 1}
};

const liveB = {
    ...liveA,
    metadata: {...liveA.metadata, resourceVersion: '200', annotations: {tick: '2'}}
};

const liveC = {
    ...liveA,
    metadata: {...liveA.metadata, resourceVersion: '300', annotations: {tick: '3'}}
};

describe('YamlEditor live freeze', () => {
    beforeEach(() => {
        lastMonacoText = '';
        fakeModel.lines = '';
        fakeModel.setValue.mockClear();
        fakeModel.pushEditOperations.mockClear();
        fakeModel.getLinesContent.mockClear();
    });

    test('keeps snapshot YAML while editing even if live input changes', () => {
        const {rerender} = renderEditor(React.createElement(YamlEditor, {input: liveA, onSave: jest.fn()}));
        expect(lastMonacoText).toBe(dumpYaml(liveA));

        act(() => {
            fireEvent.click(screen.getByRole('button', {name: /Edit/i}));
        });
        fakeModel.lines = dumpYaml(liveA) + '# user was typing\n';

        rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: liveB, onSave: jest.fn()})));

        expect(lastMonacoText).toBe(dumpYaml(liveA));
        expect(fakeModel.pushEditOperations).not.toHaveBeenCalled();
        expect(fakeModel.lines).toContain('# user was typing');
    });

    test('Cancel discards buffered edits and later refreshes reach Monaco', () => {
        const {rerender} = renderEditor(React.createElement(YamlEditor, {input: liveA, onSave: jest.fn()}));
        act(() => {
            fireEvent.click(screen.getByRole('button', {name: /Edit/i}));
        });
        fakeModel.lines = dumpYaml(liveA) + '# user was typing\n';
        rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: liveB, onSave: jest.fn()})));

        act(() => {
            fireEvent.click(screen.getByRole('button', {name: /Cancel/i}));
        });

        // The typed text only lives in the buffer, so Cancel has to clear it explicitly.
        expect(fakeModel.lines).toBe(dumpYaml(liveB));
        expect(fakeModel.setValue).not.toHaveBeenCalled();
        expect(lastMonacoText).toBe(dumpYaml(liveB));

        rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: liveC, onSave: jest.fn()})));
        expect(lastMonacoText).toBe(dumpYaml(liveC));
    });

    test('holds the saved text until the live state reaches the saved resource version', async () => {
        const savedText = jsYaml.dump({...liveA, spec: {replicas: 3}});
        const onSave = jest.fn().mockResolvedValue({...liveA, metadata: {...liveA.metadata, resourceVersion: '250'}, spec: {replicas: 3}});
        const {rerender} = renderEditor(React.createElement(YamlEditor, {input: liveA, onSave}));

        act(() => {
            fireEvent.click(screen.getByRole('button', {name: /Edit/i}));
        });
        fakeModel.lines = savedText;

        await act(async () => {
            fireEvent.click(screen.getByRole('button', {name: /Save/i}));
        });

        expect(onSave).toHaveBeenCalled();
        const [patch] = onSave.mock.calls[0];
        expect(patch).not.toContain('resourceVersion');

        // Live state is still older than the save. Handing Monaco new text would wipe what was just
        // saved, so input.text has to stay put and leave the saved buffer on screen.
        rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: liveB, onSave})));
        expect(lastMonacoText).not.toBe(dumpYaml(liveB));
        expect(lastMonacoText).toBe(dumpYaml(liveA));
        expect(fakeModel.lines).toBe(savedText);
    });

    test('resumes live refresh once the live state reaches the saved resource version', async () => {
        const onSave = jest.fn().mockResolvedValue({...liveA, metadata: {...liveA.metadata, resourceVersion: '250'}, spec: {replicas: 3}});
        const {rerender} = renderEditor(React.createElement(YamlEditor, {input: liveA, onSave}));

        act(() => {
            fireEvent.click(screen.getByRole('button', {name: /Edit/i}));
        });
        fakeModel.lines = jsYaml.dump({...liveA, spec: {replicas: 3}});
        await act(async () => {
            fireEvent.click(screen.getByRole('button', {name: /Save/i}));
        });

        // Resource version 300 is past the saved 250, so this live state includes the save.
        const settled = {...liveC, spec: {replicas: 3}};
        rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: settled, onSave})));
        expect(lastMonacoText).toBe(dumpYaml(settled));

        // Whatever the cluster says now is shown as-is, including a later change to the same field.
        const changedElsewhere = {...liveC, metadata: {...liveC.metadata, resourceVersion: '400'}, spec: {replicas: 7}};
        rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: changedElsewhere, onSave})));
        expect(lastMonacoText).toBe(dumpYaml(changedElsewhere));
    });

    test('shows whatever the cluster reports if a rejected value never reaches the saved version', async () => {
        const onSave = jest.fn().mockResolvedValue({...liveA, metadata: {...liveA.metadata, resourceVersion: '250'}, spec: {replicas: 3}});
        const {rerender} = renderEditor(React.createElement(YamlEditor, {input: liveA, onSave}));

        act(() => {
            fireEvent.click(screen.getByRole('button', {name: /Edit/i}));
        });
        fakeModel.lines = jsYaml.dump({...liveA, spec: {replicas: 3}});
        await act(async () => {
            fireEvent.click(screen.getByRole('button', {name: /Save/i}));
        });

        // A controller rewrote the value, so the live state at the saved version disagrees with the
        // edit. The cluster's answer wins rather than the edit being kept on screen.
        const rewritten = {...liveC, spec: {replicas: 1}};
        rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: rewritten, onSave})));
        expect(lastMonacoText).toBe(dumpYaml(rewritten));
    });

    test('stops waiting if the live state never reaches the saved resource version', async () => {
        jest.useFakeTimers();
        try {
            const onSave = jest.fn().mockResolvedValue({...liveA, metadata: {...liveA.metadata, resourceVersion: '999'}, spec: {replicas: 3}});
            const {rerender} = renderEditor(React.createElement(YamlEditor, {input: liveA, onSave}));

            act(() => {
                fireEvent.click(screen.getByRole('button', {name: /Edit/i}));
            });
            fakeModel.lines = jsYaml.dump({...liveA, spec: {replicas: 3}});
            await act(async () => {
                fireEvent.click(screen.getByRole('button', {name: /Save/i}));
            });

            act(() => {
                jest.advanceTimersByTime(4999);
            });
            rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: liveB, onSave})));
            expect(lastMonacoText).toBe(dumpYaml(liveA));

            act(() => {
                jest.advanceTimersByTime(1);
            });
            rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: liveC, onSave})));

            expect(lastMonacoText).toBe(dumpYaml(liveC));
        } finally {
            jest.useRealTimers();
        }
    });

    test('resumes immediately when the save response carries no resource version', async () => {
        const onSave = jest.fn().mockResolvedValue({replicas: 3});
        const {rerender} = renderEditor(React.createElement(YamlEditor, {input: liveA, onSave}));

        act(() => {
            fireEvent.click(screen.getByRole('button', {name: /Edit/i}));
        });
        fakeModel.lines = jsYaml.dump({...liveA, spec: {replicas: 3}});
        await act(async () => {
            fireEvent.click(screen.getByRole('button', {name: /Save/i}));
        });

        rerender(React.createElement(Context.Provider, {value: mockContext}, React.createElement(YamlEditor, {input: liveC, onSave})));
        expect(lastMonacoText).toBe(dumpYaml(liveC));
    });
});
