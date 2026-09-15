import * as React from 'react';

import type * as monacoEditor from 'monaco-editor';
import {applyEditorInput, EditorInput} from './monaco-apply-input';
import {services} from '../services';
import {getTheme, useSystemTheme} from '../utils';

export type {EditorInput};

export interface MonacoProps {
    minHeight?: number;
    vScrollBar: boolean;
    editor?: {
        options?: monacoEditor.editor.IEditorOptions;
        input: EditorInput;
        getApi?: (api: monacoEditor.editor.IEditor) => any;
    };
}

const DEFAULT_LINE_HEIGHT = 18;

const MonacoEditorLazy = React.lazy(() =>
    import('monaco-editor').then(monaco => {
        const Component = (props: MonacoProps) => {
            const [height, setHeight] = React.useState(0);
            const [theme, setTheme] = React.useState('dark');

            React.useEffect(() => {
                const destroySystemThemeListener = useSystemTheme(systemTheme => {
                    if (theme === 'auto') {
                        monaco.editor.setTheme(systemTheme === 'dark' ? 'vs-dark' : 'vs');
                    }
                });

                return () => {
                    destroySystemThemeListener();
                };
            }, [theme]);

            React.useEffect(() => {
                const subscription = services.viewPreferences.getPreferences().subscribe(preferences => {
                    setTheme(preferences.theme);

                    monaco.editor.setTheme(getTheme(preferences.theme) === 'dark' ? 'vs-dark' : 'vs');
                });

                return () => {
                    subscription.unsubscribe();
                };
            }, []);

            return (
                <div
                    style={{
                        height: `${Math.max(props.minHeight || 0, height + 100)}px`,
                        overflowY: 'hidden'
                    }}
                    ref={el => {
                        if (el) {
                            const container = el as {
                                editorApi?: monacoEditor.editor.IStandaloneCodeEditor;
                                prevEditorInput?: EditorInput;
                            };
                            if (props.editor) {
                                if (!container.editorApi) {
                                    const editor = monaco.editor.create(el, {
                                        ...props.editor.options,
                                        value: props.editor.input.text,
                                        language: props.editor.input.language,
                                        scrollBeyondLastLine: props.vScrollBar,
                                        scrollbar: {
                                            alwaysConsumeMouseWheel: false,
                                            vertical: props.vScrollBar ? 'visible' : 'hidden'
                                        }
                                    });

                                    container.editorApi = editor;
                                    container.prevEditorInput = props.editor.input;
                                } else {
                                    applyEditorInput(monaco, container.editorApi, container.prevEditorInput, props.editor.input);
                                    container.prevEditorInput = props.editor.input;
                                }

                                const lineCount = container.editorApi.getModel()?.getLineCount() ?? 0;
                                setHeight(lineCount * DEFAULT_LINE_HEIGHT);
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
            default: Component
        };
    })
);

export const MonacoEditor = (props: MonacoProps) => (
    <React.Suspense fallback={<div>Loading...</div>}>
        <MonacoEditorLazy {...props} />
    </React.Suspense>
);
