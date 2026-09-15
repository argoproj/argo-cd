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
// Replacing the model via setModel resets scroll; setValue + restoreViewState keeps the view put.
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
