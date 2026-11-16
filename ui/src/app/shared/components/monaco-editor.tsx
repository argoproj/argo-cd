import * as React from 'react';

import type * as monacoEditor from 'monaco-editor';
import {applyEditorInput, EditorInput} from './monaco-apply-input';
import {services} from '../services';
import {getTheme, createSystemThemeListener} from '../utils';

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

function heightForLineCount(lineCount: number, minHeight?: number) {
    const newHeight = lineCount * DEFAULT_LINE_HEIGHT + 50;
    return Math.max(minHeight || 0, newHeight > window.innerHeight * 0.95 ? window.innerHeight * 0.95 : newHeight);
}

const MonacoEditorLazy = React.lazy(() =>
    import('monaco-editor').then(monaco => {
        const Component = (props: MonacoProps) => {
            const [height, setHeight] = React.useState(0);
            const [theme, setTheme] = React.useState('dark');
            const editorApiRef = React.useRef<monacoEditor.editor.IStandaloneCodeEditor | null>(null);
            const containerRef = React.useRef<HTMLElement | null>(null);

            React.useEffect(() => {
                const destroySystemThemeListener = createSystemThemeListener(systemTheme => {
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

            React.useEffect(() => {
                const onResize = () => {
                    editorApiRef.current?.layout();
                };
                window.addEventListener('resize', onResize);
                return () => {
                    window.removeEventListener('resize', onResize);
                };
            }, []);

            // Re-layout on container resize so the editor isn't left collapsed until a later re-render.
            React.useEffect(() => {
                if (typeof ResizeObserver === 'undefined' || !containerRef.current) {
                    return undefined;
                }
                const observer = new ResizeObserver(() => {
                    editorApiRef.current?.layout();
                });
                observer.observe(containerRef.current);
                return () => {
                    observer.disconnect();
                };
            }, []);

            return (
                <div
                    style={{
                        height: `${Math.max(props.minHeight || 0, height)}px`,
                        overflowY: 'hidden'
                    }}
                    ref={el => {
                        if (el) {
                            containerRef.current = el;
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
                                            vertical: props.vScrollBar ? 'auto' : 'hidden'
                                        }
                                    });

                                    container.editorApi = editor;
                                    container.prevEditorInput = props.editor.input;
                                    editorApiRef.current = editor;
                                } else {
                                    applyEditorInput(monaco, container.editorApi, container.prevEditorInput, props.editor.input);
                                    container.prevEditorInput = props.editor.input;
                                }

                                const lineCount = container.editorApi.getModel()?.getLineCount() ?? 0;
                                setHeight(heightForLineCount(lineCount, props.minHeight));
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
