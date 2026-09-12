import type * as monacoEditor from 'monaco-editor';

export interface EditorInput {
    text: string;
    language?: string;
}

export interface MonacoModelFactory {
    editor: {
        createModel(value: string, language?: string): monacoEditor.editor.ITextModel;
    };
}

export function isEqualInput(first?: EditorInput, second?: EditorInput) {
    return first && second && first.text === second.text && (first.language || '') === (second.language || '');
}

// Replace a document's contents in place. setValue would reset the view and scroll the editor back to
// the top, and editor.executeEdits is a no-op while the editor is readOnly, so edit the model itself.
export function replaceModelText(model: monacoEditor.editor.ITextModel, text: string): void {
    model.pushEditOperations([], [{range: model.getFullModelRange(), text}], () => null);
}

// Update an existing Monaco editor's document without treating a live-text refresh as a new file.
// Only the incoming props are compared, never the buffer, so text the user is still typing survives.
export function applyEditorInput(monaco: MonacoModelFactory, editor: monacoEditor.editor.IStandaloneCodeEditor, prev: EditorInput | undefined, next: EditorInput): void {
    if (isEqualInput(prev, next)) {
        return;
    }

    const viewState = editor.saveViewState();
    const model = editor.getModel();
    const languageChanged = (prev?.language || '') !== (next.language || '');

    if (model && !languageChanged) {
        replaceModelText(model, next.text);
    } else {
        const newModel = monaco.editor.createModel(next.text, next.language);
        editor.setModel(newModel);
        model?.dispose();
    }

    if (viewState) {
        editor.restoreViewState(viewState);
    }
}
