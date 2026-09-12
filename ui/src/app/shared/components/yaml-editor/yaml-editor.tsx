import {ErrorNotification, NotificationType} from 'argo-ui';
import type * as monacoEditor from 'monaco-editor';
import * as React from 'react';
import {useContext, useEffect, useState, useRef} from 'react';

import {Context} from '../../context';
import {MonacoEditor} from '../monaco-editor';
import {replaceModelText} from '../monaco-apply-input';
import {buildYamlMergePatch, cloneAndDumpYaml, dumpYaml, isEmptyPatch} from './yaml-merge-patch';
import {hasReachedResourceVersion, resourceVersionOf} from './resource-version';

// How long the document stays held after a save before live refreshes resume regardless. Bounded so
// a resource that never reports the saved version, because it was deleted or the watch stalled,
// cannot leave the editor stuck on stale text.
const PENDING_SAVE_TIMEOUT_MS = 5000;

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
    const initialSnapshot = props.initialEditMode && props.input ? cloneAndDumpYaml(props.input) : null;
    const [editing, setEditing] = useState(!!props.initialEditMode);
    const [frozenYaml, setFrozenYaml] = useState<string | null>(initialSnapshot?.yaml ?? null);
    const [pendingResourceVersion, setPendingResourceVersion] = useState<string | null>(null);
    const modelRef = useRef<monacoEditor.editor.ITextModel | null>(null);
    const snapshotRef = useRef<T | null>(initialSnapshot?.snapshot ?? null);

    // A save is not visible in the live state until the cluster reports it back. Redrawing before
    // then would replace what was just saved with a resource that predates it, so hold the document
    // still instead. Nothing is invented: the text on screen is what was saved, and it is replaced
    // by the cluster's own answer as soon as the live resource reaches the saved version.
    const waitingForSave = pendingResourceVersion != null;
    if (waitingForSave && !editing && hasReachedResourceVersion(props.input, pendingResourceVersion)) {
        setPendingResourceVersion(null);
        setFrozenYaml(null);
    }

    const yamlText = frozenYaml != null ? frozenYaml : dumpYaml(props.input);

    useEffect(() => {
        if (!waitingForSave) {
            return;
        }
        const timeout = setTimeout(() => {
            setPendingResourceVersion(null);
            setFrozenYaml(frozen => (editing ? frozen : null));
        }, PENDING_SAVE_TIMEOUT_MS);
        return () => clearTimeout(timeout);
    }, [waitingForSave, editing]);

    const beginEdit = () => {
        // Already frozen means a save is still in flight, so keep that document and its base rather
        // than recapturing from a live state that does not have the saved change yet.
        if (frozenYaml == null && props.input) {
            const captured = cloneAndDumpYaml(props.input);
            snapshotRef.current = captured.snapshot;
            setFrozenYaml(captured.yaml);
        }
        setEditing(true);
    };

    const endEdit = () => {
        snapshotRef.current = null;
        setPendingResourceVersion(null);
        setFrozenYaml(null);
        setEditing(false);
    };

    const handleSave = async () => {
        try {
            const yaml = modelRef.current!.getLinesContent().join('\n');
            const base = snapshotRef.current ?? props.input;
            const patch = buildYamlMergePatch(base, yaml);
            try {
                const saved = await props.onSave?.(JSON.stringify(patch || {}), 'application/merge-patch+json');
                if (saved === true) {
                    // The caller unmounted us, so there is nothing left to update.
                    return;
                }
                const savedResourceVersion = isEmptyPatch(patch) ? null : resourceVersionOf(saved);
                if (savedResourceVersion) {
                    // Leave the saved text in place until the live state catches up to this version.
                    // The snapshot stays too, so reopening Edit before then still diffs against the
                    // state the buffer was built from instead of reverting fields to stale values.
                    setPendingResourceVersion(savedResourceVersion);
                    setEditing(false);
                } else {
                    endEdit();
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
        // The abandoned edits only exist in the buffer, so the props-level refresh cannot clear them.
        if (modelRef.current) {
            replaceModelText(modelRef.current, dumpYaml(props.input));
        }
        endEdit();
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
                        <button onClick={beginEdit} className='argo-button argo-button--base'>
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
