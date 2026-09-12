import type * as monacoEditor from 'monaco-editor';
import {applyEditorInput, EditorInput, MonacoModelFactory} from './monaco-apply-input';

const FULL_RANGE = {startLineNumber: 1, startColumn: 1, endLineNumber: 99, endColumn: 1};

function createFakeModel(initialText: string, languageId = 'yaml') {
    const model = {
        text: initialText,
        languageId,
        setValue: jest.fn((value: string) => {
            model.text = value;
        }),
        getFullModelRange: jest.fn(() => FULL_RANGE as unknown as monacoEditor.Range),
        pushEditOperations: jest.fn((_before: unknown, edits: {text: string}[]) => {
            model.text = edits[0].text;
            return null;
        }),
        getLanguageId: jest.fn(() => model.languageId),
        getLineCount: jest.fn(() => model.text.split('\n').length),
        dispose: jest.fn()
    };
    return model;
}

function createFakeEditor(model: ReturnType<typeof createFakeModel> | null) {
    const viewState = {scroll: 42};
    let currentModel = model as unknown as monacoEditor.editor.ITextModel | null;
    const editor = {
        saveViewState: jest.fn(() => viewState as unknown as monacoEditor.editor.ICodeEditorViewState),
        restoreViewState: jest.fn(),
        getModel: jest.fn(() => currentModel),
        setModel: jest.fn((next: monacoEditor.editor.ITextModel | null) => {
            currentModel = next;
        }),
        // Monaco refuses editor-level edits while readOnly is set, so this must never be used.
        executeEdits: jest.fn(() => false)
    };
    return {editor: editor as unknown as monacoEditor.editor.IStandaloneCodeEditor, mocks: editor, viewState};
}

function createFakeMonaco(created: ReturnType<typeof createFakeModel>[]) {
    const monaco: MonacoModelFactory = {
        editor: {
            createModel: jest.fn((value: string, language?: string) => {
                const model = createFakeModel(value, language);
                created.push(model);
                return model as unknown as monacoEditor.editor.ITextModel;
            })
        }
    };
    return monaco;
}

describe('applyEditorInput', () => {
    const prev: EditorInput = {text: 'apiVersion: v1\nkind: ConfigMap\n', language: 'yaml'};

    test('text change edits the same model in place and restores view state', () => {
        const model = createFakeModel(prev.text, 'yaml');
        const {editor, mocks, viewState} = createFakeEditor(model);
        const created: ReturnType<typeof createFakeModel>[] = [];
        const monaco = createFakeMonaco(created);
        const next: EditorInput = {text: 'apiVersion: v1\nkind: ConfigMap\nmetadata:\n  annotations:\n    tick: "1"\n', language: 'yaml'};

        applyEditorInput(monaco, editor, prev, next);

        expect(model.pushEditOperations).toHaveBeenCalledWith([], [{range: FULL_RANGE, text: next.text}], expect.any(Function));
        expect(model.text).toBe(next.text);
        expect(mocks.setModel).not.toHaveBeenCalled();
        expect(monaco.editor.createModel).not.toHaveBeenCalled();
        expect(mocks.saveViewState).toHaveBeenCalled();
        expect(mocks.restoreViewState).toHaveBeenCalledWith(viewState);
        expect(model.dispose).not.toHaveBeenCalled();
        expect(mocks.saveViewState.mock.invocationCallOrder[0]).toBeLessThan(model.pushEditOperations.mock.invocationCallOrder[0]);
        expect(model.pushEditOperations.mock.invocationCallOrder[0]).toBeLessThan(mocks.restoreViewState.mock.invocationCallOrder[0]);
    });

    test('updates the document without setValue or editor-level edits, which readOnly would block', () => {
        const model = createFakeModel(prev.text, 'yaml');
        const {editor, mocks} = createFakeEditor(model);
        const monaco = createFakeMonaco([]);
        const next: EditorInput = {text: 'kind: Secret\n', language: 'yaml'};

        applyEditorInput(monaco, editor, prev, next);

        expect(model.setValue).not.toHaveBeenCalled();
        expect(mocks.executeEdits).not.toHaveBeenCalled();
        expect(model.text).toBe(next.text);
    });

    test('unchanged input is a no-op', () => {
        const model = createFakeModel(prev.text, 'yaml');
        const {editor, mocks} = createFakeEditor(model);
        const monaco = createFakeMonaco([]);

        applyEditorInput(monaco, editor, prev, {...prev});

        expect(model.pushEditOperations).not.toHaveBeenCalled();
        expect(mocks.setModel).not.toHaveBeenCalled();
        expect(mocks.saveViewState).not.toHaveBeenCalled();
        expect(mocks.restoreViewState).not.toHaveBeenCalled();
    });

    test('language change replaces the model, disposes the old one, and restores view state', () => {
        const model = createFakeModel(prev.text, 'yaml');
        const {editor, mocks, viewState} = createFakeEditor(model);
        const created: ReturnType<typeof createFakeModel>[] = [];
        const monaco = createFakeMonaco(created);
        const next: EditorInput = {text: '{"kind":"ConfigMap"}', language: 'json'};

        applyEditorInput(monaco, editor, prev, next);

        expect(model.pushEditOperations).not.toHaveBeenCalled();
        expect(monaco.editor.createModel).toHaveBeenCalledWith(next.text, 'json');
        expect(mocks.setModel).toHaveBeenCalledWith(created[0]);
        expect(model.dispose).toHaveBeenCalled();
        expect(mocks.restoreViewState).toHaveBeenCalledWith(viewState);
        expect(mocks.saveViewState.mock.invocationCallOrder[0]).toBeLessThan(mocks.setModel.mock.invocationCallOrder[0]);
        expect(mocks.setModel.mock.invocationCallOrder[0]).toBeLessThan(mocks.restoreViewState.mock.invocationCallOrder[0]);
    });

    test('discards in-progress buffer text in favor of the incoming cluster YAML', () => {
        const model = createFakeModel('user was typing this and has not saved');
        const {editor} = createFakeEditor(model);
        const monaco = createFakeMonaco([]);
        const next: EditorInput = {text: 'kind: ConfigMap\n', language: 'yaml'};

        applyEditorInput(monaco, editor, prev, next);

        expect(model.pushEditOperations).toHaveBeenCalledWith([], [{range: FULL_RANGE, text: next.text}], expect.any(Function));
        expect(model.text).toBe(next.text);
    });
});
