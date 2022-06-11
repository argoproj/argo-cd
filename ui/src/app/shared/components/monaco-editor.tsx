import * as React from 'react';

import * as monacoEditor from 'monaco-editor';

export interface EditorInput {
    text: string;
    language?: string;
}

export interface MonacoProps {
    minHeight?: number;
    editor?: {
        options?: monacoEditor.editor.IEditorOptions;
        input: EditorInput;
        getApi?: (api: monacoEditor.editor.IEditor) => any;
    };
}

function IsEqualInput(first?: EditorInput, second?: EditorInput) {
    return first && second && first.text === second.text && (first.language || '') === (second.language || '');
}

const DEFAULT_LINE_HEIGHT = 18;

const MonacoEditorLazy = React.lazy(() =>
    import('monaco-editor').then(monaco => {
        require('monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution.js');

        const component = (props: MonacoProps) => {
            const [height, setHeight] = React.useState(0);

            return (
                <div
                    style={{
                        height: `${Math.max(props.minHeight || 0, height + 100)}px`,
                        overflowY: 'hidden'
                    }}
                    ref={el => {
                        if (el) {
                            const container = el as {
                                editorApi?: monacoEditor.editor.IEditor;
                                prevEditorInput?: EditorInput;
                            };
                            if (props.editor) {
                                if (!container.editorApi) {
                                    const editor = monaco.editor.create(el, {
                                        ...props.editor.options,
                                        scrollBeyondLastLine: false,
                                        scrollbar: {
                                            handleMouseWheel: false,
                                            vertical: 'hidden'
                                        }
                                    });

                                    container.editorApi = editor;
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
                    }}
                />
            );
        };

        return {
            default: component
        };
    })
);

export const MonacoEditor = (props: MonacoProps) => (
    <React.Suspense fallback={<div>Loading...</div>}>
        <MonacoEditorLazy {...props} />
    </React.Suspense>
);
