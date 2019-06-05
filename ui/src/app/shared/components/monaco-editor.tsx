import * as React from 'react';

import * as monacoEditor from 'monaco-editor';

export interface EditorInput { text: string; language?: string; }

export interface MonacoProps {
    minHeight?: number;
    editor?: {
        options?: monacoEditor.editor.IEditorOptions;
        input: EditorInput;
        getApi?: (api: monacoEditor.editor.IEditor) => any;
    };

    diffEditor?: {
        options?: monacoEditor.editor.IDiffEditorOptions;
        original: EditorInput;
        modified: EditorInput;
        getApi?: (api: monacoEditor.editor.IDiffEditor) => any;
    };
}

function IsEqualInput(first?: EditorInput, second?: EditorInput) {
    return first && second && first.text === second.text && (first.language || '') === (second.language || '');
}

const DEFAULT_LINE_HEIGHT = 18;

const MonacoEditorLazy = React.lazy(() => import('monaco-editor').then((monaco) => {
    require('monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution.js');

    const component = (props: MonacoProps) => {
        const [ height, setHeight ] = React.useState(0);

        return (
            <div style={{ height: `${ Math.max(props.minHeight || 0, height)}px` }} ref={(el) => {
                if (el) {
                    const container = el as {
                        diffApi?: monacoEditor.editor.IDiffEditor;
                        editorApi?: monacoEditor.editor.IEditor;
                        prevEditorInput?: EditorInput;
                    };
                    if (props.diffEditor) {
                        if (!container.diffApi) {
                            container.diffApi = monaco.editor.createDiffEditor(el, props.diffEditor.options);
                        }
                        const originalModel = monaco.editor.createModel(props.diffEditor.original.text, props.diffEditor.original.language);
                        const modifiedModel = monaco.editor.createModel(props.diffEditor.modified.text, props.diffEditor.modified.language);
                        container.diffApi.setModel({original: originalModel, modified: modifiedModel});

                        const lineCount = Math.max(originalModel.getLineCount(), modifiedModel.getLineCount());
                        setHeight(lineCount * DEFAULT_LINE_HEIGHT);
                        container.diffApi.updateOptions(props.diffEditor.options);
                        container.diffApi.layout();
                        if (props.diffEditor.getApi) {
                            props.diffEditor.getApi(container.diffApi);
                        }
                    } else if (props.editor) {
                        if (!container.editorApi) {
                            container.editorApi = monaco.editor.create(el, props.editor.options);
                        }

                        const model = monaco.editor.createModel(props.editor.input.text, props.editor.input.language);
                        const lineCount = model.getLineCount();
                        setHeight(lineCount * DEFAULT_LINE_HEIGHT);

                        if (!IsEqualInput(container.prevEditorInput, props.editor.input)) {
                            container.prevEditorInput = props.editor.input;
                            container.editorApi.setModel(model);
                        }
                        container.editorApi.updateOptions(props.editor.options);
                        container.editorApi.layout();
                        if (props.editor.getApi) {
                            props.editor.getApi(container.editorApi);
                        }
                    }
                }
            }}/>
        );
    };

    return {
        default: component,
    };
}));

export const MonacoEditor = (props: MonacoProps) => (
    <React.Suspense fallback={<div>Loading...</div>}>
        <MonacoEditorLazy {...props}/>
    </React.Suspense>
);
