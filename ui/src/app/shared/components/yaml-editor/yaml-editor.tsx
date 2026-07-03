import {ErrorNotification, NotificationType} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as monacoEditor from 'monaco-editor';
import * as React from 'react';
import {useContext, useState, useRef} from 'react';

import {Context} from '../../context';
import {MonacoEditor} from '../monaco-editor';

const jsonMergePatch = require('json-merge-patch');
require('./yaml-editor.scss');

interface YamlEditorProps<T> {
    input: T;
    hideModeButtons?: boolean;
    initialEditMode?: boolean;
    vScrollbar?: boolean;
    enableWordWrap?: boolean;
    onSave?: (patch: string, patchType: string) => Promise<any>;
    onCancel?: () => any;
    minHeight?: number;
}

export function YamlEditor<T>(props: YamlEditorProps<T>) {
    const ctx = useContext(Context);
    const [editing, setEditing] = useState(!!props.initialEditMode);
    const modelRef = useRef<monacoEditor.editor.ITextModel | null>(null);

    const yamlText = props.input ? jsYaml.dump(props.input) : '';

    const handleSave = async () => {
        try {
            const updated = jsYaml.load(modelRef.current!.getLinesContent().join('\n'));
            const patch = jsonMergePatch.generate(props.input, updated);
            try {
                const unmounted = await props.onSave?.(JSON.stringify(patch || {}), 'application/merge-patch+json');
                if (unmounted !== true) {
                    setEditing(false);
                }
            } catch (e) {
                ctx.notifications.show({
                    content: (
                        <div className='yaml-editor__error'>
                            <ErrorNotification title='Unable to save changes' e={e} />
                        </div>
                    ),
                    type: NotificationType.Error
                });
            }
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to validate changes' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    const handleCancel = () => {
        modelRef.current?.setValue(jsYaml.dump(props.input));
        setEditing(false);
        props.onCancel?.();
    };

    return (
        <div className='yaml-editor'>
            {!props.hideModeButtons && (
                <div className='yaml-editor__buttons'>
                    {editing ? (
                        <>
                            <button onClick={handleSave} className='argo-button argo-button--base'>
                                Save
                            </button>{' '}
                            <button onClick={handleCancel} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </>
                    ) : (
                        <button onClick={() => setEditing(true)} className='argo-button argo-button--base'>
                            Edit
                        </button>
                    )}
                </div>
            )}
            <MonacoEditor
                minHeight={props.minHeight}
                vScrollBar={props.vScrollbar}
                editor={{
                    input: {text: yamlText, language: 'yaml'},
                    options: {
                        readOnly: !editing,
                        minimap: {enabled: false},
                        wordWrap: props.enableWordWrap ? 'on' : 'off'
                    },
                    getApi: api => {
                        modelRef.current = api.getModel() as monacoEditor.editor.ITextModel;
                    }
                }}
            />
        </div>
    );
}
