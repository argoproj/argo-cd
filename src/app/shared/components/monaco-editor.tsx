import * as React from 'react';

import * as monacoEditor from 'monaco-editor';

export interface MonacoProps {
    editor?: {
        options?: monacoEditor.editor.IEditorOptions;
        input: { text: string, language?: string };
    };

    diffEditor?: {
        options?: monacoEditor.editor.IDiffEditorOptions;
        original: { text: string, language?: string };
        modified: { text: string, language?: string };
    };
}

const DEFAULT_LINE_HEIGHT = 18;

const MonacoEditorLazy = React.lazy(() => import('monaco-editor/esm/vs/editor/editor.api.js').then((monaco) => {
    require('monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution.js');

    const component = (props: MonacoProps) => {
        const [ height, setHeight ] = React.useState(0);

        return (
            <div style={{ height: `${height}px` }} ref={(el) => {
                if (el) {
                    const container = el as { diffApi?: monacoEditor.editor.IDiffEditor; editorApi?: monacoEditor.editor.IEditor };
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
                    } else if (props.editor) {
                        if (!container.editorApi) {
                            container.editorApi = monaco.editor.create(el, props.editor.options);
                        }
                        const model = monaco.editor.createModel(props.editor.input.text, props.editor.input.language);
                        const lineCount = model.getLineCount();
                        setHeight(lineCount * DEFAULT_LINE_HEIGHT);
                        container.editorApi.setModel(model);
                        container.editorApi.updateOptions(props.editor.options);
                        container.editorApi.layout();
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
