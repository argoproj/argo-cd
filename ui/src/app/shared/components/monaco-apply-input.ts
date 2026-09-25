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

// Update an existing Monaco editor's document without treating a live-text refresh as a new file.
// Only the incoming props are compared, never the buffer, so text the user is still typing survives.
// setValue (not an edit operation) so undo cannot resurrect a stale refresh and save it over the cluster.
export function applyEditorInput(monaco: MonacoModelFactory, editor: monacoEditor.editor.IStandaloneCodeEditor, prev: EditorInput | undefined, next: EditorInput): void {
    if (isEqualInput(prev, next)) {
        return;
    }

    const viewState = editor.saveViewState();
    const model = editor.getModel();
    const languageChanged = (prev?.language || '') !== (next.language || '');

    if (model && !languageChanged) {
        model.setValue(next.text);
    } else {
        const newModel = monaco.editor.createModel(next.text, next.language);
        editor.setModel(newModel);
        model?.dispose();
    }

    if (viewState) {
        editor.restoreViewState(viewState);
    }
}
